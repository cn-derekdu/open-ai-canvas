package app

import (
	"context"
	"errors"
	"testing"
)

func TestProviderTaskRecoveryContextSurvivesClientCancellation(t *testing.T) {
	type contextKey string
	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), contextKey("trace"), "trace-1"))
	cancelParent()

	recovery, cancelRecovery := providerTaskRecoveryContext(parent)
	defer cancelRecovery()
	if recovery.Err() != nil {
		t.Fatalf("recovery context inherited cancellation: %v", recovery.Err())
	}
	if recovery.Value(contextKey("trace")) != "trace-1" {
		t.Fatal("recovery context did not preserve request values")
	}
	if _, ok := recovery.Deadline(); !ok {
		t.Fatal("recovery context has no bounded deadline")
	}
}

func TestRetryableProtocolMediaDownload(t *testing.T) {
	active := context.Background()
	for _, err := range []error{
		errors.New("net/http: TLS handshake timeout"),
		errors.New("read: connection reset by peer"),
		errors.New("unexpected EOF"),
		// 任务 context 仍然有效时收到的 deadline exceeded，命中的是单次下载的独立上限：
		// 必须可重试，否则 providerDownloadTimeout 只是把失败提前，重试窗口依然用不上。
		context.DeadlineExceeded,
	} {
		if !retryableProtocolMediaDownload(active, err) {
			t.Fatalf("retryableProtocolMediaDownload(%v) = false", err)
		}
	}

	expired, cancelExpired := context.WithTimeout(context.Background(), 0)
	defer cancelExpired()
	if expired.Err() == nil {
		t.Fatal("测试前提：任务 context 应当已到点")
	}
	// 任务级超时/取消说明时间已经不属于本次任务，重试没有意义。
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, errors.New("HTTP 404")} {
		if retryableProtocolMediaDownload(expired, err) {
			t.Fatalf("retryableProtocolMediaDownload(%v) = true，任务 context 已到点不该重试", err)
		}
	}
}
