package pipeline

import (
	"context"
	"reflect"
	"testing"
)

type decoderFunc struct {
	push  func(context.Context, int) ([]string, error)
	close func(context.Context) ([]string, error)
}

func (d decoderFunc) Push(ctx context.Context, ev int) ([]string, error) {
	return d.push(ctx, ev)
}

func (d decoderFunc) Close(ctx context.Context) ([]string, error) {
	return d.close(ctx)
}

func TestTransform(t *testing.T) {
	in := make(chan int, 2)
	in <- 1
	in <- 2
	close(in)

	out := Transform[int, string](context.Background(), in, decoderFunc{
		push: func(ctx context.Context, ev int) ([]string, error) {
			return []string{string(rune('a' + ev - 1))}, nil
		},
		close: func(ctx context.Context) ([]string, error) {
			return []string{"done"}, nil
		},
	})
	var got []string
	for item := range out {
		if item.Err != nil {
			t.Fatal(item.Err)
		}
		got = append(got, item.Value)
	}
	if want := []string{"a", "b", "done"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Transform = %v, want %v", got, want)
	}
}
