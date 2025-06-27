package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/vemolista/learn-s3/internal/auth"
	"github.com/vemolista/learn-s3/internal/database"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIdParams := r.PathValue("videoID")
	videoUuid, err := uuid.Parse(videoIdParams)
	if videoIdParams == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	jwtToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Could not get jwt", err)
		return
	}

	userId, err := auth.ValidateJWT(jwtToken, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Could not validate jwt", err)
		return
	}

	maxSize := 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxSize))

	videoMeta, err := cfg.db.GetVideo(videoUuid)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not get video metadata", err)
		return
	}

	if videoMeta.UserID != userId {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to access video of another user", err)
		return
	}

	formFile, formFileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not get video from request", err)
		return
	}

	contentTypeHeader := formFileHeader.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse media type", err)
		return
	}

	if mediaType != "video/mp4" {
		errMsg := fmt.Sprintf("Only 'video/mp4' allowed, got %s", mediaType)
		respondWithError(w, http.StatusBadRequest, errMsg, err)
		return
	}

	extension := "." + strings.Split(mediaType, "/")[1]

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create temporary upload file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, formFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not copy file", err)
		return
	}

	outputFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not reprocess video", err)
		return
	}
	defer os.Remove(outputFilePath)

	processedFile, err := os.Open(outputFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not open processed video", err)
		return
	}
	defer processedFile.Close()

	ratio, err := getVideoAspectRatio(processedFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not get video aspect ratio", err)
		return
	}
	prefix := getS3KeyPrefix(ratio)

	_, err = processedFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not set read offset", err)
		return
	}

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could generate random bytes", err)
		return
	}

	randomStr := base64.RawURLEncoding.EncodeToString(randomBytes)
	key := fmt.Sprintf("%s/%s%s", prefix, randomStr, extension)

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        processedFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not upload object to s3", err)
		return
	}

	videoUrl := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, key)

	err = cfg.db.UpdateVideo(database.Video{
		ID:           videoMeta.ID,
		CreatedAt:    videoMeta.CreatedAt,
		UpdatedAt:    videoMeta.UpdatedAt,
		ThumbnailURL: videoMeta.ThumbnailURL,
		VideoURL:     &videoUrl,
		CreateVideoParams: database.CreateVideoParams{
			UserID:      videoMeta.UserID,
			Title:       videoMeta.Title,
			Description: videoMeta.Description,
		},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not update video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, nil)
}
