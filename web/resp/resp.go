// Package resp HTTP 状态码风格的统一响应结构（Code int 标识结果，可区分 401/403/500 等场景）。
//
// 与 web/respok（布尔风格，Ok bool 只区分成功/失败）互为替代，按前端需要选择。
package resp

import "net/http"

type Resp[T any] struct {
	Code int    `json:"code"`
	Data T      `json:"data,omitempty"`
	Msg  string `json:"msg"`
}

type PageResp[T any] struct {
	Resp[T]
	Total int64 `json:"total"`
}

const (
	MsgOk = "success"
)

func Success[T any](data T) Resp[T] {
	return Resp[T]{
		Code: http.StatusOK,
		Data: data,
		Msg:  MsgOk,
	}
}

func SuccessNoData() Resp[any] {
	return Resp[any]{
		Code: http.StatusOK,
		Msg:  MsgOk,
	}
}

func Fail(code int, msg string) Resp[any] {
	return Resp[any]{
		Code: code,
		Msg:  msg,
	}
}

func FailWithMsg(msg string) Resp[any] {
	return Fail(http.StatusInternalServerError, msg)
}

func FailWithError(err error) Resp[any] {
	return Fail(http.StatusInternalServerError, err.Error())
}
