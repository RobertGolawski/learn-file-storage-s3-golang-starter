package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	const maxMem = 10 << 20
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	r.ParseMultipartForm(maxMem)

	fileData, h, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse image", err)
		return
	}
	defer fileData.Close()

	mt := h.Header.Get("Content-Type")
	if mt == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type", err)
		return
	}

	mimeType, _, err := mime.ParseMediaType(mt)
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Incorrect media type", err)
		return
	}

	rByte := make([]byte, 32)
	_, err = rand.Read(rByte)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Problem with encoding", err)
		return
	}

	randID := base64.RawURLEncoding.EncodeToString(rByte)

	filepath := getAssetPath(randID, mt)
	diskPath := cfg.getAssetDiskPath(filepath)
	file, err := os.Create(diskPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}
	defer file.Close()
	_, err = io.Copy(file, fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write to file", err)
		return
	}

	tURL := cfg.getAssetURL(filepath)

	vidResp, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't fetch vid id", err)
		return
	}
	if userID != vidResp.UserID {
		respondWithError(w, http.StatusUnauthorized, "Unauthed", err)
		return
	}

	vidResp.ThumbnailURL = &tURL

	err = cfg.db.UpdateVideo(vidResp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update vid", err)
		return
	}

	respondWithJSON(w, http.StatusOK, vidResp)
}
