// Package health verifica a prontidão sem depender de transporte HTTP.
package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var (
	ErrSchemaMissing = errors.New("schema outguessed ausente")
	ErrStopping      = errors.New("aplicação em encerramento")
)

// Error conserva a causa para errors.Is/As sem expô-la em logs ou mensagens.
type Error struct {
	Dependency string
	Err        error
}

func (e *Error) Error() string {
	if errors.Is(e.Err, ErrSchemaMissing) {
		return "PostgreSQL: schema outguessed ausente"
	}
	return e.Dependency + " indisponível"
}

func (e *Error) Unwrap() error { return e.Err }

type SchemaChecker interface {
	CheckSchema(context.Context) (bool, error)
}

type Pinger interface {
	Ping(context.Context) error
}

type Service struct {
	schema  SchemaChecker
	redis   Pinger
	timeout time.Duration
	logger  *slog.Logger
	mu      sync.Mutex
	checked bool
	ready   bool
	stopped bool
}

func New(schema SchemaChecker, redis Pinger, timeout time.Duration, logger *slog.Logger) *Service {
	return &Service{schema: schema, redis: redis, timeout: timeout, logger: logger}
}

// Check consulta as duas dependências sob um único orçamento, sem reutilizar resultados antigos.
func (s *Service) Check(ctx context.Context) error {
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return ErrStopping
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	err := s.check(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrStopping
	}
	ready := err == nil
	if !s.checked || s.ready != ready {
		if ready {
			s.logger.InfoContext(ctx, "Dependências disponíveis")
		} else {
			s.logger.WarnContext(ctx, "Aplicação não pronta", "error", err)
		}
	}
	s.checked, s.ready = true, ready
	return err
}

func (s *Service) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ready, err := s.schema.CheckSchema(ctx)
	if err != nil {
		return &Error{Dependency: "PostgreSQL", Err: err}
	}
	if !ready {
		return &Error{Dependency: "PostgreSQL", Err: ErrSchemaMissing}
	}
	if err := s.redis.Ping(ctx); err != nil {
		return &Error{Dependency: "Redis", Err: err}
	}
	return ctx.Err()
}

// Wait repete verificações somente enquanto o contexto de inicialização permitir.
func (s *Service) Wait(ctx context.Context) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("aguardar dependências: %w", errors.Join(err, last))
		}
		last = s.Check(ctx)
		if last == nil {
			return nil
		}
		if errors.Is(last, ErrStopping) {
			return last
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("aguardar dependências: %w", errors.Join(ctx.Err(), last))
		case <-timer.C:
		}
	}
}

// Stop impede novas verificações e invalida resultados de verificações em andamento.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	s.ready = false
}
