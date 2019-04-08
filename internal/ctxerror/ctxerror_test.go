package ctxerror

import (
	"errors"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	type args struct {
		msg string
		ctx []interface{}
	}
	tests := []struct {
		name string
		args args
		want CtxError
	}{
		{
			name: "Empty",
			args: args{msg: "", ctx: []interface{}{}},
			want: &ctxError{msg: "", ctx: map[string]interface{}{}},
		},
		{
			name: "Regular",
			args: args{msg: "omg", ctx: []interface{}{"wtf", 1, "bbq", 2}},
			want: &ctxError{msg: "omg", ctx: map[string]interface{}{"wtf": 1, "bbq": 2}},
		},
		{
			name: "Truncated",
			args: args{
				msg: "omg",
				ctx: []interface{}{"wtf", 1, "bbq" /* missing value... */},
			},
			want: &ctxError{
				msg: "omg",
				ctx: map[string]interface{}{"wtf": 1, "bbq": /* becomes */ nil},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.msg, tt.args.ctx...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_ctxError_updateCtx(t *testing.T) {
	tests := []struct {
		name          string
		before, after map[string]interface{}
		delta         []interface{}
	}{
		{
			name:   "Empty",
			before: map[string]interface{}{"omg": 1, "wtf": 2, "bbq": 3},
			delta:  []interface{}{},
			after:  map[string]interface{}{"omg": 1, "wtf": 2, "bbq": 3},
		},
		{
			name:   "Regular",
			before: map[string]interface{}{"omg": 1, "wtf": 2, "bbq": 3},
			delta:  []interface{}{"omg", 10, "wtf", 20},
			after:  map[string]interface{}{"omg": 10, "wtf": 20, "bbq": 3},
		},
		{
			name:   "Truncated",
			before: map[string]interface{}{"omg": 1, "wtf": 2, "bbq": 3},
			delta:  []interface{}{"omg", 10, "wtf" /* missing value... */},
			after:  map[string]interface{}{"omg": 10, "wtf": /* becomes */ nil, "bbq": 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ctxError{msg: tt.name, ctx: tt.before}
			e.updateCtx(tt.delta...)
			if !reflect.DeepEqual(e.ctx, tt.after) {
				t.Errorf("expected ctx %#v != %#v seen", tt.after, e.ctx)
			}
		})
	}
}

func Test_ctxError_Error(t *testing.T) {
	type fields struct {
		msg string
		ctx map[string]interface{}
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "AllEmpty",
			fields: fields{msg: "", ctx: map[string]interface{}{}},
			want:   "",
		},
		{
			name:   "CtxEmpty",
			fields: fields{msg: "omg", ctx: map[string]interface{}{}},
			want:   "omg",
		},
		{
			name:   "MsgEmpty",
			fields: fields{msg: "", ctx: map[string]interface{}{"wtf": "bbq"}},
			want:   ", wtf=\"bbq\"",
		},
		{
			name:   "Regular",
			fields: fields{msg: "omg", ctx: map[string]interface{}{"wtf": "bbq"}},
			want:   "omg, wtf=\"bbq\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ctxError{
				msg: tt.fields.msg,
				ctx: tt.fields.ctx,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_ctxError_Message(t *testing.T) {
	type fields struct {
		msg string
		ctx map[string]interface{}
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "AllEmpty",
			fields: fields{msg: "", ctx: map[string]interface{}{}},
			want:   "",
		},
		{
			name:   "CtxEmpty",
			fields: fields{msg: "omg", ctx: map[string]interface{}{}},
			want:   "omg",
		},
		{
			name:   "MsgEmpty",
			fields: fields{msg: "", ctx: map[string]interface{}{"wtf": "bbq"}},
			want:   "",
		},
		{
			name:   "Regular",
			fields: fields{msg: "omg", ctx: map[string]interface{}{"wtf": "bbq"}},
			want:   "omg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ctxError{
				msg: tt.fields.msg,
				ctx: tt.fields.ctx,
			}
			if got := e.Message(); got != tt.want {
				t.Errorf("Message() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_ctxError_Contexts(t *testing.T) {
	type fields struct {
		msg string
		ctx map[string]interface{}
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]interface{}
	}{
		{
			name:   "Empty",
			fields: fields{msg: "", ctx: map[string]interface{}{}},
			want:   map[string]interface{}{},
		},
		{
			name: "Regular",
			fields: fields{
				msg: "",
				ctx: map[string]interface{}{"omg": 1, "wtf": 2, "bbq": 3},
			},
			want: map[string]interface{}{"omg": 1, "wtf": 2, "bbq": 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ctxError{
				msg: tt.fields.msg,
				ctx: tt.fields.ctx,
			}
			if got := e.Contexts(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Contexts() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_ctxError_WithCause(t *testing.T) {
	type fields struct {
		msg string
		ctx map[string]interface{}
	}
	type args struct {
		c error
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   CtxError
	}{
		{
			name: "CtxError",
			fields: fields{
				msg: "hello",
				ctx: map[string]interface{}{"omg": 1, "wtf": 2},
			},
			args: args{c: &ctxError{
				msg: "world",
				ctx: map[string]interface{}{"wtf": 20, "bbq": 30},
			}},
			want: &ctxError{
				msg: "hello: world",
				ctx: map[string]interface{}{"omg": 1, "wtf": 20, "bbq": 30},
			},
		},
		{
			name: "RegularError",
			fields: fields{
				msg: "hello",
				ctx: map[string]interface{}{"omg": 1, "wtf": 2},
			},
			args: args{c: errors.New("world")},
			want: &ctxError{
				msg: "hello: world",
				ctx: map[string]interface{}{"omg": 1, "wtf": 2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ctxError{
				msg: tt.fields.msg,
				ctx: tt.fields.ctx,
			}
			if got := e.WithCause(tt.args.c); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithCause() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_ctxError_Log15(t *testing.T) {
	type fields struct {
		msg string
		ctx map[string]interface{}
	}
	type want struct {
		msg string
		ctx []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name:   "Empty",
			fields: fields{msg: "", ctx: map[string]interface{}{}},
			want:   want{msg: "", ctx: nil},
		},
		{
			name:   "Regular",
			fields: fields{msg: "hello", ctx: map[string]interface{}{"omg": 1}},
			want:   want{msg: "hello", ctx: []interface{}{"omg", 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			f := func(msg string, ctx ...interface{}) {
				called = true
				if msg != tt.want.msg {
					t.Errorf("expected message %#v != %#v seen",
						tt.want.msg, msg)
				}
				if !reflect.DeepEqual(ctx, tt.want.ctx) {
					t.Errorf("expected ctx %#v != %#v seen", ctx, tt.want.ctx)
				}
			}
			e := &ctxError{
				msg: tt.fields.msg,
				ctx: tt.fields.ctx,
			}
			e.Log15(f)
			if !called {
				t.Error("logging func not called")
			}
		})
	}
}

func TestLog15(t *testing.T) {
	type args struct {
		e error
	}
	type want struct {
		msg string
		ctx []interface{}
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Regular",
			args: args{e: errors.New("hello")},
			want: want{msg: "hello", ctx: nil},
		},
		{
			name: "CtxError",
			args: args{e: &ctxError{
				msg: "hello",
				ctx: map[string]interface{}{"omg": 1},
			}},
			want: want{msg: "hello", ctx: []interface{}{"omg", 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			f := func(msg string, ctx ...interface{}) {
				called = true
				if msg != tt.want.msg {
					t.Errorf("expected message %#v != %#v seen",
						tt.want.msg, msg)
				}
				if !reflect.DeepEqual(ctx, tt.want.ctx) {
					t.Errorf("expected ctx %#v != %#v seen",
						tt.want.ctx, ctx)
				}
			}
			Log15(f, tt.args.e)
			if !called {
				t.Errorf("logging func not called")
			}
		})
	}
}
