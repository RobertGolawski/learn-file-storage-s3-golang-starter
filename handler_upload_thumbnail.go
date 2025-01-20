package main

import (
	"fmt"
	"io"
	"log"
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

	readData, err := io.ReadAll(fileData)
	if err != nil || readData == nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read file", err)
		return
	}

	// imgAsString := base64.StdEncoding.EncodeToString(readData)

	extension := mt[6:]
	log.Print(extension)
	// filepath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s.%s", videoID, extension))
	filepath := fmt.Sprintf("./assets/%s.%v", videoID, extension)
	file, err := os.Create(filepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}

	_, err = io.Copy(file, fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write to file", err)
		return
	}

	// dURL := fmt.Sprintf("data:%s;base64,%s", mt, imgAsString)

	tURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, videoID, extension)

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
