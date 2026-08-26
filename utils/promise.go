package utils

import (
	"context"
	"errors"
	"time"
)

type PromisePool struct {
	sem chan struct{}
}

func NewPromisePool(maxConcurrent int) *PromisePool {
	return &PromisePool{sem: make(chan struct{}, maxConcurrent)}
}

func (p *PromisePool) acquire() { p.sem <- struct{}{} }
func (p *PromisePool) release() { <-p.sem }

func PromiseAll[T any](
	parent context.Context,
	timeout time.Duration,
	pool *PromisePool,
	tasks []func(context.Context) (T, error),
) ([]T, []error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	n := len(tasks)
	results := make([]T, n)
	errs := make([]error, n)
	done := make([]bool, n)

	if n == 0 {
		return results, errs
	}

	type tr struct {
		i   int
		val T
		err error
	}

	compCh := make(chan tr, n)

	// Lanzamos todas las tareas
	for i, task := range tasks {
		if pool != nil {
			pool.acquire()
		}
		i, task := i, task
		go func() {
			defer func() {
				if pool != nil {
					pool.release()
				}
			}()
			val, err := task(ctx)
			select {
			case compCh <- tr{i: i, val: val, err: err}:
			case <-ctx.Done(): // ya expiró, no bloqueamos
			}
		}()
	}

	remaining := n
	for remaining > 0 {
		select {
		case <-ctx.Done():
			// Timeout o cancel externo → devolvemos enseguida
			for i := 0; i < n; i++ {
				if !done[i] {
					var zero T
					results[i] = zero
					errs[i] = ctx.Err()
				}
			}
			return results, errs

		case r := <-compCh:
			if !done[r.i] {
				results[r.i] = r.val
				errs[r.i] = r.err
				done[r.i] = true
				remaining--
			}
		}
	}

	return results, errs
}

func PromiseAny[T any](
	ctx context.Context,
	timeout time.Duration,
	pool *PromisePool,
	tasks []func(context.Context) (T, error),
) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type taskResult struct {
		value T
		err   error
	}
	resultCh := make(chan taskResult, len(tasks))
	var zero T
	if len(tasks) == 0 {
		return zero, errors.New("no tasks provided")
	}
	for _, task := range tasks {
		if pool != nil {
			pool.acquire()
		}
		go func(fn func(context.Context) (T, error)) {
			if pool != nil {
				defer pool.release()
			}
			res, err := fn(ctx)
			resultCh <- taskResult{value: res, err: err}
		}(task)
	}

	errs := make([]error, 0, len(tasks))
	for range tasks {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case result := <-resultCh:
			if result.err == nil {
				return result.value, nil
			}
			errs = append(errs, result.err)
		}
	}
	return zero, errors.Join(errs...)
}
