package hmr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
)

// buildSet assembles a minimal valid NormalizedSet with three records.
func buildSet() *norm.NormalizedSet {
	return &norm.NormalizedSet{
		ValidatorList:     norm.Record{Key: []byte("validator-list"), Value: []byte{0xc0}},
		ShardState:        norm.Record{Key: []byte("ss\x0b\xba"), Value: []byte("shardbytes")},
		RewardAccumulator: norm.Record{Key: append([]byte("blk-rwd-"), 0, 0, 0, 0, 0, 0, 0, 7), Value: []byte{0x2a}},
	}
}

func TestEncodeGoldenVector(t *testing.T) {
	set := buildSet()
	var anchorSHA [32]byte
	for i := range anchorSHA {
		anchorSHA[i] = byte(i)
	}
	got, err := Encode(set, anchorSHA)
	if err != nil {
		t.Fatal(err)
	}
	// Hand-compute the expected container. Records sort bytewise by key:
	// "blk-rwd-\x00...\x07" < "ss\x0b\xba" < "validator-list".
	var want bytes.Buffer
	want.Write([]byte("HMR1"))
	want.Write([]byte{0, 0, 0, 1})
	want.Write(anchorSHA[:])
	want.Write([]byte{0, 0, 0, 0, 0, 0, 0, 3})
	frame := func(k, v []byte) {
		var k4 [4]byte
		binary.BigEndian.PutUint32(k4[:], uint32(len(k)))
		want.Write(k4[:])
		want.Write(k)
		var v8 [8]byte
		binary.BigEndian.PutUint64(v8[:], uint64(len(v)))
		want.Write(v8[:])
		want.Write(v)
	}
	frame(append([]byte("blk-rwd-"), 0, 0, 0, 0, 0, 0, 0, 7), []byte{0x2a})
	frame([]byte("ss\x0b\xba"), []byte("shardbytes"))
	frame([]byte("validator-list"), []byte{0xc0})
	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("encoder drifted from golden:\n got %x\nwant %x", got, want.Bytes())
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	set := buildSet()
	set.DVL = []norm.Record{{Key: append([]byte("dvl"), bytes.Repeat([]byte{1}, 20)...), Value: []byte{0xc1, 0x02}}}
	var anchorSHA [32]byte
	enc, err := Encode(set, anchorSHA)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Records) != 4 {
		t.Fatalf("decoded %d records, want 4", len(dec.Records))
	}
	// encode(decode(x)) == x is guaranteed by monotone key order.
	for i := 1; i < len(dec.Records); i++ {
		if bytes.Compare(dec.Records[i-1].Key, dec.Records[i].Key) >= 0 {
			t.Fatalf("decoded records not strictly increasing at %d", i)
		}
	}
}

func TestPackageDigestChangesOnFlip(t *testing.T) {
	set := buildSet()
	var sha [32]byte
	a, _ := Encode(set, sha)
	set.ShardState.Value = []byte("shardbyteS") // flip one byte
	b, _ := Encode(set, sha)
	if bytes.Equal(a, b) {
		t.Fatal("flipping a value byte must change the package bytes")
	}
}

func TestDecoderRejections(t *testing.T) {
	set := buildSet()
	var sha [32]byte
	good, _ := Encode(set, sha)

	t.Run("bad-magic", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		bad[0] = 'X'
		if _, err := Decode(bad); !errors.Is(err, ErrBadMagic) {
			t.Fatalf("want ErrBadMagic, got %v", err)
		}
	})
	t.Run("bad-version", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		bad[7] = 2
		if _, err := Decode(bad); !errors.Is(err, ErrBadVersion) {
			t.Fatalf("want ErrBadVersion, got %v", err)
		}
	})
	t.Run("trailing-byte", func(t *testing.T) {
		bad := append(append([]byte(nil), good...), 0xff)
		if _, err := Decode(bad); !errors.Is(err, ErrTrailingBytes) {
			t.Fatalf("want ErrTrailingBytes, got %v", err)
		}
	})
	t.Run("truncated-value", func(t *testing.T) {
		bad := good[:len(good)-1]
		if _, err := Decode(bad); !errors.Is(err, ErrTruncated) {
			t.Fatalf("want ErrTruncated, got %v", err)
		}
	})
	t.Run("count-too-high", func(t *testing.T) {
		bad := append([]byte(nil), good...)
		binary.BigEndian.PutUint64(bad[40:48], 99)
		if _, err := Decode(bad); !errors.Is(err, ErrCountMismatch) {
			t.Fatalf("want ErrCountMismatch, got %v", err)
		}
	})
	t.Run("duplicate-key", func(t *testing.T) {
		dup := buildDuplicate(sha)
		if _, err := Decode(dup); !errors.Is(err, ErrDuplicateKey) {
			t.Fatalf("want ErrDuplicateKey, got %v", err)
		}
	})
	t.Run("out-of-order", func(t *testing.T) {
		ooo := buildOutOfOrder(sha)
		if _, err := Decode(ooo); !errors.Is(err, ErrOutOfOrder) {
			t.Fatalf("want ErrOutOfOrder, got %v", err)
		}
	})
}

func buildRaw(sha [32]byte, count uint64, frames ...[]byte) []byte {
	var b bytes.Buffer
	b.Write([]byte("HMR1"))
	b.Write([]byte{0, 0, 0, 1})
	b.Write(sha[:])
	var c8 [8]byte
	binary.BigEndian.PutUint64(c8[:], count)
	b.Write(c8[:])
	for _, f := range frames {
		b.Write(f)
	}
	return b.Bytes()
}

func frame(k, v []byte) []byte {
	var out bytes.Buffer
	var k4 [4]byte
	binary.BigEndian.PutUint32(k4[:], uint32(len(k)))
	out.Write(k4[:])
	out.Write(k)
	var v8 [8]byte
	binary.BigEndian.PutUint64(v8[:], uint64(len(v)))
	out.Write(v8[:])
	out.Write(v)
	return out.Bytes()
}

func buildDuplicate(sha [32]byte) []byte {
	return buildRaw(sha, 2, frame([]byte("k"), []byte("a")), frame([]byte("k"), []byte("b")))
}

func buildOutOfOrder(sha [32]byte) []byte {
	return buildRaw(sha, 2, frame([]byte("kb"), []byte("a")), frame([]byte("ka"), []byte("b")))
}

// FuzzDecode ensures the decoder never panics on arbitrary input.
func FuzzDecode(f *testing.F) {
	set := buildSet()
	var sha [32]byte
	good, _ := Encode(set, sha)
	f.Add(good)
	f.Add([]byte("HMR1"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Decode(data)
	})
}
