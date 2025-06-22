package main

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vemolista/learn-s3/internal/auth"
	"github.com/vemolista/learn-s3/internal/database"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	const maxMemory = 10 << 20 // 10 * 1024 * 1024

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error when parsing form", err)
		return
	}

	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting form file", err)
		return
	}

	mediaTypeHeader := fileHeader.Header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(mediaTypeHeader)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing media type", err)
		return
	}

	if mediaType != "image/png" && mediaType != "image/jpeg" {
		respondWithError(w, http.StatusBadRequest, "Only png or jpeg files allowed", err)
		return
	}

	extension := "." + strings.Split(mediaType, "/")[1]

	videoMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video", err)
		return
	}

	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Cannot access videos of other users", err)
		return
	}

	path := filepath.Join(cfg.assetsRoot, videoMeta.ID.String()+extension)
	newFile, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}

	defer newFile.Close()
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying file", err)
		return
	}

	path = "/" + path

	videoToUpdate := database.Video{
		ID:           videoMeta.ID,
		CreatedAt:    videoMeta.CreatedAt,
		UpdatedAt:    time.Now(),
		ThumbnailURL: &path,
		VideoURL:     videoMeta.VideoURL,
		CreateVideoParams: database.CreateVideoParams{
			Title:       videoMeta.Title,
			Description: videoMeta.Description,
			UserID:      videoMeta.UserID,
		},
	}

	err = cfg.db.UpdateVideo(videoToUpdate)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoToUpdate)
}
