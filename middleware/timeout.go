package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/kataras/iris/v12"
)

func HandlerTimeout(ctx iris.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx.Request().Context(), 30*time.Second)
	defer cancel()
	//st := time.Now().Unix()
	ctx.ResetRequest(ctx.Request().WithContext(timeoutCtx))
	go func() {
		<-timeoutCtx.Done()
		//ed := time.Now().Unix()
		//log.Println("timeout request spend:", ed-st)
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			ctx.StatusCode(iris.StatusRequestTimeout)
			ctx.StopExecution()
		}
	}()
	ctx.Next()
}
