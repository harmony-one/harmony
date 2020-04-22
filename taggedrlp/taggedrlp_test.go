package taggedrlp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/rlp"
)

const RecordV0Tag = LegacyTag

type RecordV0 struct {
	Name    string
	Address string
}

var RecordV0Type = reflect.TypeOf(RecordV0{})

var RecordV0Value = RecordV0{
	Name:    "Harmony",
	Address: "349 Martens Ave",
}

func RecordV0Factory() interface{} {
	return new(RecordV0)
}

func RecordV0Factory2() interface{} {
	return &RecordV0{Name: "Unnamed"}
}

func InvalidRecordV0Factory() interface{} {
	return RecordV0{}
}

const RecordV1Tag = "RecordV1"

type RecordV1 struct {
	Name    string
	Address string
	Phone   string
}

var RecordV1Type = reflect.TypeOf(RecordV1{})

var RecordV1Value = RecordV1{
	Name:    "Harmony",
	Address: "349 Martens Ave",
	Phone:   "+14153234477",
}

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	switch {
	case reg.typeForTag == nil:
		t.Error("typeForTag is nil")
	case len(reg.typeForTag) != 0:
		t.Error("typeForTag is not empty")
	}
	switch {
	case reg.tagForType == nil:
		t.Error("tagForType is nil")
	case len(reg.tagForType) != 0:
		t.Error("tagForType is not empty")
	}
	switch {
	case reg.factoryForType == nil:
		t.Error("factoryForType is nil")
	case len(reg.factoryForType) != 0:
		t.Error("factoryForType is not empty")
	}
}

func TestPkgPathQualifiedName(t *testing.T) {
	type args struct {
		name    string
		pkgPath string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Packageless", args{name: "copy", pkgPath: ""}, `copy`},
		{"Packaged", args{name: "Int", pkgPath: "math/big"}, `"math/big".Int`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PkgPathQualifiedName(tt.args.name, tt.args.pkgPath); got != tt.want {
				t.Errorf("PkgPathQualifiedName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPkgPathQualifiedTypeName(t *testing.T) {
	type args struct {
		typ reflect.Type
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Packageless", args{reflect.TypeOf(int(0))}, `int`},
		{"Packaged", args{reflect.TypeOf(big.Int{})}, `"math/big".Int`},
		{"Internal", args{reflect.TypeOf(errors.New("OMG"))}, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PkgPathQualifiedTypeName(tt.args.typ); got != tt.want {
				t.Errorf("PkgPathQualifiedTypeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeName(t *testing.T) {
	type args struct {
		typ reflect.Type
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Packageless", args{reflect.TypeOf(int(0))}, `int`},
		{"Packaged", args{reflect.TypeOf(big.Int{})}, `"math/big".Int`},
		{"Internal", args{reflect.TypeOf(errors.New("OMG"))}, `*errors.errorString`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TypeName(tt.args.typ); got != tt.want {
				t.Errorf("TypeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_Register(t *testing.T) {
	type fields struct {
		typeForTag typeForTag
		tagForType tagForType
	}
	type args struct {
		tag       Tag
		prototype interface{}
	}
	tests := []struct {
		name    string
		pre     fields
		args    args
		wantErr error
		post    fields
	}{
		{
			name: "Succeessful",
			pre: fields{
				typeForTag{},
				tagForType{},
			},
			args: args{tag: RecordV0Tag, prototype: RecordV0Value},
			post: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
		},
		{
			name: "PointerDeref",
			pre: fields{
				typeForTag{},
				tagForType{},
			},
			args: args{tag: RecordV0Tag, prototype: &RecordV0Value},
			post: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
		},
		{
			name: "Idempotent",
			pre: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
			args: args{tag: RecordV0Tag, prototype: RecordV0Value},
			post: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
		},
		{
			name: "RebindingSameTagNotAllowed",
			pre: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
			args:    args{tag: RecordV0Tag, prototype: RecordV1Value},
			wantErr: TagAlreadyBoundToType{RecordV0Tag, RecordV0Type},
			post: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
		},
		{
			name: "RebindingSameTypeNotAllowed",
			pre: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
			args:    args{tag: RecordV1Tag, prototype: RecordV0Value},
			wantErr: TypeAlreadyBoundToTag{RecordV0Type, RecordV0Tag},
			post: fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &Registry{
				typeForTag: tt.pre.typeForTag,
				tagForType: tt.pre.tagForType,
			}
			err := reg.Register(tt.args.tag, tt.args.prototype)
			if err != tt.wantErr {
				t.Errorf("Register() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(reg.typeForTag, tt.post.typeForTag) {
				t.Errorf("Registry typeForTag mismatch, expected %#v, got %#v",
					tt.post.typeForTag, reg.typeForTag)
			}
			if !reflect.DeepEqual(reg.tagForType, tt.post.tagForType) {
				t.Errorf("Registry tagForType mismatch, expected %#v, got %#v",
					tt.post.tagForType, reg.tagForType)
			}
		})
	}
}

func TestRegistry_MustRegister(t *testing.T) {
	type fields struct {
		typeForTag typeForTag
		tagForType tagForType
	}
	type args struct {
		tag       Tag
		prototype interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr error
	}{
		{
			"Succeessful",
			fields{
				typeForTag{},
				tagForType{},
			},
			args{tag: RecordV0Tag, prototype: RecordV0Value},
			nil,
		},
		{
			"PanicOnError",
			fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
			},
			args{tag: RecordV0Tag, prototype: RecordV1Value},
			TagAlreadyBoundToType{RecordV0Tag, RecordV0Type},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered != tt.wantErr {
					t.Errorf("MustRegister() recovered %v, want %v",
						recovered, tt.wantErr)
				}
			}()
			reg := &Registry{
				typeForTag: tt.fields.typeForTag,
				tagForType: tt.fields.tagForType,
			}
			reg.MustRegister(tt.args.tag, tt.args.prototype)
		})
	}
}

func TestRegistry_AddFactory(t *testing.T) {
	type fields struct {
		typeForTag     typeForTag
		tagForType     tagForType
		factoryForType factoryForType
	}
	type args struct {
		factory Factory
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr error
	}{
		{
			"Successful",
			fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
				factoryForType{},
			},
			args{factory: RecordV0Factory},
			nil,
		},
		{
			"IdempotentNotAllowed",
			fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
				factoryForType{RecordV0Type: RecordV0Factory},
			},
			args{factory: RecordV0Factory},
			TypeAlreadyHasFactory{RecordV0Type},
		},
		{
			"RebindingSameTypeNotAllowed",
			fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
				factoryForType{RecordV0Type: RecordV0Factory},
			},
			args{factory: RecordV0Factory2},
			TypeAlreadyHasFactory{RecordV0Type},
		},
		{
			"NonPointerReturningFactoryNotAllowed",
			fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
				factoryForType{},
			},
			args{factory: InvalidRecordV0Factory},
			InvalidFactoryReturnType{RecordV0Type},
		},
		{
			"UnregisteredTypeNotAllowed",
			fields{
				typeForTag{},
				tagForType{},
				factoryForType{},
			},
			args{factory: RecordV0Factory},
			TypeNotRegistered{RecordV0Type},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &Registry{
				typeForTag:     tt.fields.typeForTag,
				tagForType:     tt.fields.tagForType,
				factoryForType: tt.fields.factoryForType,
			}
			if err := reg.AddFactory(tt.args.factory); err != tt.wantErr {
				t.Errorf("AddFactory() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_MustAddFactory(t *testing.T) {
	type fields struct {
		typeForTag     typeForTag
		tagForType     tagForType
		factoryForType factoryForType
	}
	type args struct {
		factory Factory
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantPanic bool
	}{
		{
			"Successful",
			fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
				factoryForType{},
			},
			args{factory: RecordV0Factory},
			false,
		},
		{
			"PanicOnError",
			fields{
				typeForTag{RecordV0Tag: RecordV0Type},
				tagForType{RecordV0Type: RecordV0Tag},
				factoryForType{RecordV0Type: RecordV0Factory},
			},
			args{factory: RecordV0Factory},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &Registry{
				typeForTag:     tt.fields.typeForTag,
				tagForType:     tt.fields.tagForType,
				factoryForType: tt.fields.factoryForType,
			}
			defer func() {
				recovered := recover()
				if (recovered != nil) != tt.wantPanic {
					t.Errorf("MustAddFactory() recovered %v, wantPanic %v",
						recovered, tt.wantPanic)
				}
			}()
			reg.MustAddFactory(tt.args.factory)
		})
	}
}

var EnvelopeType = reflect.TypeOf(Envelope{})

const EnvelopeTag = "Env"

var Float64Type = reflect.TypeOf(float64(0))

const Float64Tag = "Float64"

func makeRecordRegistry() *Registry {
	return &Registry{
		typeForTag{
			RecordV0Tag: RecordV0Type,
			RecordV1Tag: RecordV1Type,
			EnvelopeTag: EnvelopeType,
			Float64Tag:  Float64Type,
		},
		tagForType{
			RecordV0Type: RecordV0Tag,
			RecordV1Type: RecordV1Tag,
			EnvelopeType: EnvelopeTag,
			Float64Type:  Float64Tag,
		},
		factoryForType{},
	}
}

var RecordV0WithoutEnvelope = []byte("" +
	// BEGIN 24-byte RecordV0 list
	"\xd8" +
	// 7-byte Name
	"\x87Harmony" +
	// 15-byte Address
	"\x8f349 Martens Ave" +
	// END RecordV0 list
	"")

var RecordV1WithEnvelope = []byte("" +
	// BEGIN 55-byte envelope list
	"\xf7" +
	// 7-byte envelope signature
	"\x87HmnyTgd" +
	// 8-byte type tag
	"\x88RecordV1" +
	// BEGIN 37-byte RecordV1 list
	"\xe5" +
	// 7-byte Name
	"\x87Harmony" +
	// 15-byte Address
	"\x8f349 Martens Ave" +
	// 12-byte Phone
	"\x8c+14153234477" +
	// END RecordV1 list
	// END envelope list
	"")

func TestRegistry_Encode(t *testing.T) {
	type args struct {
		d interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantEnc []byte
		wantErr error
	}{
		{
			"LegacyUntaggedWithoutEnvelope",
			args{RecordV0Value},
			RecordV0WithoutEnvelope,
			nil,
		},
		{
			"TaggedWithEnvelope",
			args{RecordV1Value},
			RecordV1WithEnvelope,
			nil,
		},
		{
			"PointerIsDerefed",
			args{&RecordV1Value},
			RecordV1WithEnvelope,
			nil,
		},
		{
			"UnknownType",
			args{0},
			nil,
			TypeNotRegistered{reflect.TypeOf(0)},
		},
		{
			"UnencodableType",
			args{float64(0)},
			nil,
			UnencodableValue{float64(0), errors.New(
				"rlp: type float64 is not RLP-serializable")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := makeRecordRegistry()
			w := &bytes.Buffer{}
			err := reg.Encode(w, tt.args.d)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("Encode() error = %#v, want %#v", err, tt.wantErr)
			}
			if gotEnc := w.Bytes(); !bytes.Equal(gotEnc, tt.wantEnc) {
				t.Errorf("Encode() result = %x, want %x", gotEnc, tt.wantEnc)
			}
		})
	}
}

type errorChecker interface {
	CheckError(t *testing.T, err error)
}

type nilError struct{}

func (nilError) CheckError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

type errorValue struct{ want error }

func (v errorValue) CheckError(t *testing.T, err error) {
	if err != v.want {
		t.Errorf("expected error %#v, got %#v", v.want, err)
	}
}

type errorString struct{ want string }

func (s errorString) CheckError(t *testing.T, err error) {
	got := err.Error()
	if got != s.want {
		t.Errorf("expected error %#v, got %#v", s.want, got)
	}
}

func TestRegistry_Decode(t *testing.T) {
	tests := []struct {
		name         string
		b            []byte
		want         interface{}
		errorChecker errorChecker
	}{
		{
			"UntaggedIntoLegacy",
			RecordV0WithoutEnvelope,
			&RecordV0Value,
			nilError{},
		},
		{
			"TaggedWithEnvelope",
			RecordV1WithEnvelope,
			&RecordV1Value,
			nilError{},
		},
		{
			"FirstDecodeError",
			[]byte{},
			nil,
			errorValue{io.EOF},
		},
		{
			"EnvelopeSigMismatchFallbackToLegacy",
			[]byte("" +
				// BEGIN 16-byte Envelope list
				"\xd0" +
				// BEGIN 6-byte invalid Signature
				"\x86OMIGOD" +
				// BEGIN 3-byte Envelope tag
				"\x83Env" +
				// BEGIN 4-byte random string
				"\x84HELO" +
				// END Envelope list
				""),
			nil,
			errorString{"rlp: input list has too many elements for taggedrlp.RecordV0"},
		},
		{
			"UnknownTagCausesDirectError",
			[]byte("" +
				// BEGIN 16-byte Envelope list
				"\xd0" +
				// BEGIN 7-byte invalid Signature
				"\x87HmnyTgd" +
				// BEGIN 2-byte unknown tag
				"\x82NO" +
				// BEGIN 4-byte random string
				"\x84HELO" +
				// END Envelope list
				""),
			nil,
			errorValue{UnsupportedTag{"NO"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := makeRecordRegistry()
			s := rlp.NewStream(bytes.NewReader(tt.b), 0)
			got, err := reg.Decode(s)
			tt.errorChecker.CheckError(t, err)
			if !reflect.DeepEqual(tt.want, got) {
				t.Error("Encode() result mismatch")
				t.Errorf("want %#v", tt.want)
				t.Errorf("got  %#v", got)
			}
		})
	}
}

func Test_ErrorsIncludeFields(t *testing.T) {
	tests := []struct {
		err        error
		components []string
	}{
		{
			TagAlreadyBoundToType{RecordV0Tag, RecordV0Type},
			[]string{fmt.Sprintf("%#v", RecordV0Tag), TypeName(RecordV0Type)},
		},
		{
			TypeAlreadyBoundToTag{RecordV0Type, RecordV0Tag},
			[]string{fmt.Sprintf("%#v", RecordV0Tag), TypeName(RecordV0Type)},
		},
		{
			InvalidFactoryReturnType{RecordV0Type},
			[]string{TypeName(RecordV0Type)},
		},
		{
			TypeNotRegistered{RecordV0Type},
			[]string{TypeName(RecordV0Type)},
		},
		{
			TypeAlreadyHasFactory{RecordV0Type},
			[]string{TypeName(RecordV0Type)},
		},
		{
			UnencodableValue{"OMG", errors.New("WTF BBQ")},
			[]string{fmt.Sprintf("%#v", "OMG"), ": WTF BBQ"},
		},
		{
			UnsupportedTag{"OMG"},
			[]string{fmt.Sprintf("%#v", "OMG")},
		},
	}
	for _, tt := range tests {
		t.Run(reflect.TypeOf(tt.err).Name(), func(t *testing.T) {
			s := tt.err.Error()
			for _, c := range tt.components {
				if !strings.Contains(s, c) {
					t.Errorf("%#v not found in error string %#v", c, s)
				}
			}
		})
	}
}

type causer interface {
	Cause() error
}

func Test_ErrorCauseReturnsErrField(t *testing.T) {
	tests := []struct {
		err   causer
		cause error
	}{
		{
			UnencodableValue{Err: errors.New("OMG")},
			errors.New("OMG"),
		},
	}
	for _, tt := range tests {
		t.Run(reflect.TypeOf(tt.err).Name(), func(t *testing.T) {
			cause := tt.err.Cause()
			if !reflect.DeepEqual(cause, tt.cause) {
				t.Errorf("Cause() = %#v, want %#v", cause, tt.cause)
			}
		})
	}
}
