package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/labstack/echo/v5"
)

const kycChunkSize int64 = 8 << 20
const maxKYCSize int64 = 100 << 20

type beginKYCRequest struct {
	Title       string `json:"title"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	FileSize    int64  `json:"fileSize"`
}
type completeKYCRequest struct {
	Parts []struct {
		PartNumber int32  `json:"partNumber"`
		ETag       string `json:"etag"`
	} `json:"parts"`
}

func randomSlug() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *API) kycClient() (*s3.Client, *s3.PresignClient, error) {
	if a.s3Client == nil || a.config.S3Bucket == "" {
		return nil, nil, echo.NewHTTPError(http.StatusServiceUnavailable, "KYC S3 storage is not configured")
	}
	return a.s3Client, a.s3Presigner, nil
}

func (a *API) authorizeMember(c *echo.Context, memberID string) error {
	claims, err := a.requestClaims(c)
	if err != nil {
		return err
	}
	if claims.Role == "admin" || claims.Role == "staff" {
		return nil
	}
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM members m WHERE m.id=$1 AND m.user_id=$2 AND m.active)`
	if claims.Role == "promoter" {
		query = `SELECT EXISTS(SELECT 1 FROM members m JOIN promoters p ON p.id=m.promoter_id WHERE m.id=$1 AND p.user_id=$2 AND p.active)`
	}
	if err = a.service.DB.QueryRowContext(c.Request().Context(), query, memberID, claims.Subject).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return echo.NewHTTPError(http.StatusForbidden, "not authorized for this member")
	}
	return nil
}

func (a *API) beginKYCUpload(c *echo.Context) error {
	memberID := c.Param("memberId")
	if err := a.authorizeMember(c, memberID); err != nil {
		return err
	}
	var req beginKYCRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid KYC upload")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.FileName = filepath.Base(strings.TrimSpace(req.FileName))
	req.ContentType = strings.TrimSpace(req.ContentType)
	if req.Title == "" || len(req.Title) > 120 || req.FileName == "." || req.FileSize < 1 || req.FileSize > maxKYCSize {
		return echo.NewHTTPError(400, "title, filename, and a file up to 100 MiB are required")
	}
	if !strings.HasPrefix(req.ContentType, "image/") && req.ContentType != "application/pdf" {
		return echo.NewHTTPError(400, "only image and PDF documents are supported")
	}
	client, _, err := a.kycClient()
	if err != nil {
		return err
	}
	slug, err := randomSlug()
	if err != nil {
		return err
	}
	key := "kyc/" + memberID + "/" + slug
	out, err := client.CreateMultipartUpload(c.Request().Context(), &s3.CreateMultipartUploadInput{Bucket: aws.String(a.config.S3Bucket), Key: aws.String(key), ContentType: aws.String(req.ContentType), ServerSideEncryption: types.ServerSideEncryptionAes256})
	if err != nil {
		return err
	}
	claims, _ := a.requestClaims(c)
	_, err = a.service.DB.ExecContext(c.Request().Context(), `INSERT INTO kyc_upload_sessions(slug,member_id,upload_id,object_key,title,file_name,content_type,file_size,part_size,initiated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, slug, memberID, aws.ToString(out.UploadId), key, req.Title, req.FileName, req.ContentType, req.FileSize, kycChunkSize, claims.Subject)
	if err != nil {
		_, _ = client.AbortMultipartUpload(c.Request().Context(), &s3.AbortMultipartUploadInput{Bucket: aws.String(a.config.S3Bucket), Key: aws.String(key), UploadId: out.UploadId})
		return err
	}
	parts := (req.FileSize + kycChunkSize - 1) / kycChunkSize
	return c.JSON(http.StatusCreated, map[string]any{"slug": slug, "partSize": kycChunkSize, "partCount": parts})
}

func (a *API) kycPartURL(c *echo.Context) error {
	memberID, slug := c.Param("memberId"), c.Param("slug")
	if err := a.authorizeMember(c, memberID); err != nil {
		return err
	}
	var key, uploadID string
	var size, partSize int64
	if err := a.service.DB.QueryRowContext(c.Request().Context(), `SELECT object_key,upload_id,file_size,part_size FROM kyc_upload_sessions WHERE slug=$1 AND member_id=$2`, slug, memberID).Scan(&key, &uploadID, &size, &partSize); err != nil {
		return serviceError(err)
	}
	n, parseErr := strconv.Atoi(c.QueryParam("partNumber"))
	if parseErr != nil || n < 1 || int64(n) > (size+partSize-1)/partSize {
		return echo.NewHTTPError(400, "invalid part number")
	}
	_, presigner, err := a.kycClient()
	if err != nil {
		return err
	}
	result, err := presigner.PresignUploadPart(c.Request().Context(), &s3.UploadPartInput{Bucket: aws.String(a.config.S3Bucket), Key: aws.String(key), UploadId: aws.String(uploadID), PartNumber: aws.Int32(int32(n))}, func(o *s3.PresignOptions) { o.Expires = 10 * time.Minute })
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"url": result.URL})
}

func (a *API) completeKYCUpload(c *echo.Context) error {
	memberID, slug := c.Param("memberId"), c.Param("slug")
	if err := a.authorizeMember(c, memberID); err != nil {
		return err
	}
	var req completeKYCRequest
	if err := c.Bind(&req); err != nil || len(req.Parts) == 0 {
		return echo.NewHTTPError(400, "upload parts are required")
	}
	var key, uploadID, title, fileName, contentType string
	var size int64
	var uploader *string
	err := a.service.DB.QueryRowContext(c.Request().Context(), `SELECT object_key,upload_id,title,file_name,content_type,file_size,initiated_by::text FROM kyc_upload_sessions WHERE slug=$1 AND member_id=$2`, slug, memberID).Scan(&key, &uploadID, &title, &fileName, &contentType, &size, &uploader)
	if err != nil {
		return serviceError(err)
	}
	parts := make([]types.CompletedPart, 0, len(req.Parts))
	for _, part := range req.Parts {
		if part.PartNumber < 1 || strings.TrimSpace(part.ETag) == "" {
			return echo.NewHTTPError(400, "invalid part metadata")
		}
		etag := part.ETag
		parts = append(parts, types.CompletedPart{PartNumber: aws.Int32(part.PartNumber), ETag: &etag})
	}
	client, _, err := a.kycClient()
	if err != nil {
		return err
	}
	_, err = client.CompleteMultipartUpload(c.Request().Context(), &s3.CompleteMultipartUploadInput{Bucket: aws.String(a.config.S3Bucket), Key: aws.String(key), UploadId: aws.String(uploadID), MultipartUpload: &types.CompletedMultipartUpload{Parts: parts}})
	if err != nil {
		return err
	}
	head, err := client.HeadObject(c.Request().Context(), &s3.HeadObjectInput{Bucket: aws.String(a.config.S3Bucket), Key: aws.String(key)})
	if err != nil {
		return err
	}
	if aws.ToInt64(head.ContentLength) != size {
		return echo.NewHTTPError(400, "uploaded file size does not match the declared size")
	}
	var docID string
	err = a.service.DB.QueryRowContext(c.Request().Context(), `INSERT INTO kyc_documents(slug,member_id,title,file_name,content_type,file_size,object_key,uploaded_by) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid) RETURNING id::text`, slug, memberID, title, fileName, contentType, size, key, aws.ToString(uploader)).Scan(&docID)
	if err != nil {
		return err
	}
	_, err = a.service.DB.ExecContext(c.Request().Context(), `INSERT INTO audit_entries(actor_user_id,action,entity_type,entity_id,request_id,after_state) VALUES(NULLIF($1,'')::uuid,'kyc_document_uploaded','kyc_documents',$2,$3,jsonb_build_object('memberId',$4,'title',$5,'slug',$6,'fileName',$7,'contentType',$8,'fileSize',$9,'objectKey',$10))`, aws.ToString(uploader), docID, c.Request().Header.Get("X-Request-ID"), memberID, title, slug, fileName, contentType, size, key)
	if err != nil {
		return err
	}
	_, _ = a.service.DB.ExecContext(c.Request().Context(), `DELETE FROM kyc_upload_sessions WHERE slug=$1`, slug)
	return c.JSON(http.StatusCreated, map[string]any{"id": docID, "slug": slug, "title": title, "fileName": fileName, "contentType": contentType, "fileSize": size})
}

func (a *API) listKYCDocuments(c *echo.Context) error {
	memberID := c.Param("memberId")
	if err := a.authorizeMember(c, memberID); err != nil {
		return err
	}
	rows, err := a.service.DB.QueryContext(c.Request().Context(), `SELECT id::text,slug,title,file_name,content_type,file_size,uploaded_by::text,created_at FROM kyc_documents WHERE member_id=$1 ORDER BY created_at DESC`, memberID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type item struct {
		ID          string  `json:"id"`
		Slug        string  `json:"slug"`
		Title       string  `json:"title"`
		FileName    string  `json:"fileName"`
		ContentType string  `json:"contentType"`
		FileSize    int64   `json:"fileSize"`
		UploadedBy  *string `json:"uploadedBy"`
		CreatedAt   any     `json:"createdAt"`
	}
	items := []item{}
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.ID, &v.Slug, &v.Title, &v.FileName, &v.ContentType, &v.FileSize, &v.UploadedBy, &v.CreatedAt); err != nil {
			return err
		}
		items = append(items, v)
	}
	return c.JSON(http.StatusOK, items)
}

func (a *API) downloadKYCDocument(c *echo.Context) error {
	memberID, slug := c.Param("memberId"), c.Param("slug")
	if err := a.authorizeMember(c, memberID); err != nil {
		return err
	}
	var key, fileName string
	if err := a.service.DB.QueryRowContext(c.Request().Context(), `SELECT object_key,file_name FROM kyc_documents WHERE member_id=$1 AND slug=$2`, memberID, slug).Scan(&key, &fileName); err != nil {
		return serviceError(err)
	}
	_, presigner, err := a.kycClient()
	if err != nil {
		return err
	}
	out, err := presigner.PresignGetObject(c.Request().Context(), &s3.GetObjectInput{Bucket: aws.String(a.config.S3Bucket), Key: aws.String(key), ResponseContentDisposition: aws.String(`attachment; filename="` + strings.ReplaceAll(fileName, "\"", "") + `"`)}, func(o *s3.PresignOptions) { o.Expires = 5 * time.Minute })
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"url": out.URL})
}
