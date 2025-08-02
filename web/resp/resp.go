package resp

import "net/http"

type Resp[T any] struct {
	Code int    `json:"code"`
	Data T      `json:"data"`
	Msg  string `json:"msg"`
}

type PageResp[T any] struct {
	Resp[T]
	Total int64 `json:"total"`
}

func Success[T any](data T) Resp[T] {
	return Resp[T]{
		Code: http.StatusOK,
		Data: data,
		Msg:  "success",
	}
}

func Fail[T any](code int, msg string) Resp[T] {
	return Resp[T]{
		Code: code,
		Msg:  msg,
	}
}
