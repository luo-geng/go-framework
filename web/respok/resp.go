// Package respok 布尔风格统一响应结构（Ok bool 标识成功/失败）。
//
// 与 web/resp（HTTP 状态码风格，Code int 可区分 401/403/500 等场景）互为替代：
// 前端只需要两态结果时用本包，需要细分错误码时用 web/resp。
package respok

type Resp[T any] struct {
	Ok   bool   `json:"ok"`
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
		Ok:   true,
		Data: data,
		Msg:  MsgOk,
	}
}

func SuccessNoData() Resp[any] {
	return Resp[any]{
		Ok:  true,
		Msg: MsgOk,
	}
}

func Fail(msg string) Resp[any] {
	return Resp[any]{
		Ok:  false,
		Msg: msg,
	}
}

func FailWithError(err error) Resp[any] {
	return Fail(err.Error())
}
