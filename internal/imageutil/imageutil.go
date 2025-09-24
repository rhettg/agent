package imageutil

import (
	"encoding/base64"
	"strings"
)

// MimeType determines the MIME type from a filename
func MimeType(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot == -1 || dot == len(name)-1 {
		// Just a guess
		return "image/jpeg"
	}

	return "image/" + strings.ToLower(name[dot+1:])
}

// EncodeImageURL creates a base64 data URL from image data
func EncodeImageURL(mimeType string, data []byte) string {
	// Based on the python reference code in
	// https://platform.openai.com/docs/guides/vision/uploading-base-64-encoded-images
	// this should be the parallel of:
	//     base64.b64encode(image_file.read()).decode('utf-8')
	// which defaults to the standard base64 encoding.
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(dst, data)

	var imageURL strings.Builder
	imageURL.WriteString("data:")
	imageURL.WriteString(mimeType)
	imageURL.WriteString(";base64,")
	imageURL.Write(dst)

	return imageURL.String()
}
