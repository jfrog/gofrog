package filestream

type MultipartError struct {
	FileName string `json:"file_name"`
	Message  string `json:"message"`
}

func (e MultipartError) Error() string {
	return e.Message
}

func NewMultipartError(fileName, message string) *MultipartError {
	return &MultipartError{
		FileName: fileName,
		Message:  message,
	}
}
