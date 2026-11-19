package health

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type schemaFunc func(context.Context) (bool, error)

func (f schemaFunc) CheckSchema(ctx context.Context) (bool, error) { return f(ctx) }

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

func quietLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func TestCurrentReadinessRecoveryAndSanitizedLogs(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	schemaPresent := true
	var databaseErr, redisErr error
	checks, pings := 0, 0
	s := New(schemaFunc(func(context.Context) (bool, error) {
		checks++
		return schemaPresent, databaseErr
	}), pingFunc(func(context.Context) error { pings++; return redisErr }), time.Second, logger)
	if err := s.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	databaseErr = errors.New("postgres://user:secret@localhost/internal")
	if err := s.Check(context.Background()); !errors.Is(err, databaseErr) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("causa perdida ou exposta: %v", err)
	}
	databaseErr = nil
	schemaPresent = false
	if err := s.Check(context.Background()); !errors.Is(err, ErrSchemaMissing) {
		t.Fatalf("schema ausente: %v", err)
	}
	schemaPresent = true
	redisErr = errors.New("redis://:secret@localhost/internal")
	if err := s.Check(context.Background()); !errors.Is(err, redisErr) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("falha Redis não identificável: %v", err)
	}
	redisErr = nil
	if err := s.Check(context.Background()); err != nil {
		t.Fatalf("recuperação: %v", err)
	}
	if checks != 5 || pings != 3 {
		t.Fatalf("verificações atuais: postgres=%d redis=%d", checks, pings)
	}
	if strings.Contains(logs.String(), "secret") || strings.Count(logs.String(), "Dependências disponíveis") != 2 || strings.Count(logs.String(), "Aplicação não pronta") != 1 {
		t.Fatalf("logs sem deduplicação de transições ou com segredo: %s", logs.String())
	}
	s.Stop()
	if !errors.Is(s.Check(context.Background()), ErrStopping) || checks != 5 {
		t.Fatal("encerramento permitiu novas consultas")
	}
}

func TestReadinessHasOneTotalBudget(t *testing.T) {
	var firstDeadline time.Time
	s := New(schemaFunc(func(ctx context.Context) (bool, error) {
		firstDeadline, _ = ctx.Deadline()
		return true, nil
	}), pingFunc(func(ctx context.Context) error {
		deadline, _ := ctx.Deadline()
		if !deadline.Equal(firstDeadline) {
			t.Error("Redis recebeu um orçamento adicional")
		}
		<-ctx.Done()
		return ctx.Err()
	}), 20*time.Millisecond, quietLogger())
	started := time.Now()
	if err := s.Check(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("prazo: %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("verificação excedeu o orçamento")
	}
}

func TestWaitRetriesAndHonorsStartupDeadline(t *testing.T) {
	var calls atomic.Int32
	s := New(schemaFunc(func(context.Context) (bool, error) { return calls.Add(1) > 1, nil }), pingFunc(func(context.Context) error { return nil }), time.Second, quietLogger())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Wait(ctx); err != nil || calls.Load() != 2 {
		t.Fatalf("tentativa e recuperação: %v, chamadas=%d", err, calls.Load())
	}
	s = New(schemaFunc(func(ctx context.Context) (bool, error) { <-ctx.Done(); return false, ctx.Err() }), nil, time.Second, quietLogger())
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := s.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("prazo global: %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("prazo global não prevaleceu sobre o prazo da dependência")
	}
}

func TestStopInvalidatesInFlightCheck(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	s := New(schemaFunc(func(context.Context) (bool, error) {
		close(entered)
		<-release
		return true, nil
	}), pingFunc(func(context.Context) error { return nil }), time.Second, quietLogger())
	done := make(chan error, 1)
	go func() { done <- s.Check(context.Background()) }()
	<-entered
	s.Stop()
	close(release)
	if err := <-done; !errors.Is(err, ErrStopping) {
		t.Fatalf("verificação confirmou prontidão durante encerramento: %v", err)
	}
}

func TestCancelledCheckDoesNotCallDependencies(t *testing.T) {
	s := New(schemaFunc(func(context.Context) (bool, error) { t.Fatal("consulta após cancelamento"); return false, nil }), nil, time.Second, quietLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(s.Check(ctx), context.Canceled) || !errors.Is(s.Wait(ctx), context.Canceled) {
		t.Fatal("cancelamento não foi preservado")
	}
}
