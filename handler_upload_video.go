package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMem = 10 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMem)
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	vidInfo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}
	if vidInfo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorised", err)
		return
	}
	vidData, h, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse video", err)
		return
	}
	defer vidData.Close()

	mt := h.Header.Get("Content-Type")
	if mt == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type", err)
		return
	}

	mimeType, _, err := mime.ParseMediaType(mt)
	if mimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Incorrect media type", err)
		return
	}

	tmp, err := os.CreateTemp("", "temp-tubely.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error making temp file", err)
		return
	}

	defer os.Remove(tmp.Name())
	defer tmp.Close()

	_, err = io.Copy(tmp, vidData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying video", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tmp.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting aspect ratio", err)
		return
	}

	if aspectRatio == "16:9" {
		aspectRatio = "landscape"
	} else if aspectRatio == "9:16" {
		aspectRatio = "portrait"
	}

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error encoding", err)
		return
	}
	k := hex.EncodeToString(b)
	fileKey := fmt.Sprintf("%s/%s%s", aspectRatio, k, getExtension(mt))
	tmp.Seek(0, io.SeekStart)

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(fileKey),
		Body:        tmp,
		ContentType: &mimeType,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), putInput)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error putting", err)
		return
	}

	newURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileKey)

	vidInfo.VideoURL = &newURL

	log.Printf("Updading videourl to: %s", *vidInfo.VideoURL)
	cfg.db.UpdateVideo(vidInfo)

}
