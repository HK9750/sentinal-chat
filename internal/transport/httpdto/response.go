package httpdto

type Response[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

func NewSuccessResponse[T any](data T) Response[T] {
	return Response[T]{
		Success: true,
		Data:    data,
	}
}

func NewErrorResponse(err string, code string) Response[any] {
	return Response[any]{
		Success: false,
		Error:   err,
		Code:    code,
	}
}
