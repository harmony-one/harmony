package ethdb_memwrap_test

import (
	"testing"

	"github.com/harmony-one/harmony/libs/ethdb_memwrap"
	"github.com/stretchr/testify/require"
)

var key = []byte("01234567")

func TestEthWrapper(t *testing.T) {
	db := ethdb_memwrap.NewDbWrapper(nil)
	db.Wrap(true)

	t.Run("simple_get", func(t *testing.T) {
		_ = db.Put(key, []byte("abc"))
		rs, _ := db.Get(key)
		require.Equal(t, []byte("abc"), rs)
	})

	t.Run("error_on_absence", func(t *testing.T) {
		require.Panics(t, func() {
			db.Get(nil)
		})
	})

	t.Run("update_value", func(t *testing.T) {
		_ = db.Put(key, []byte("abcde"))
		rs, _ := db.Get(key)
		require.Equal(t, []byte("abcde"), rs)
	})

	t.Run("insert_second_value", func(t *testing.T) {
		key2 := append(key, '1')
		_ = db.Put(key2, []byte("abc1"))
		rs2, _ := db.Get(key2)
		require.Equal(t, []byte("abc1"), rs2)

		rs1, _ := db.Get(key)
		require.Equal(t, []byte("abcde"), rs1)
	})

	t.Run("has", func(t *testing.T) {
		key2 := append(key, '1')
		ok1, _ := db.Has(key)
		require.True(t, ok1)

		ok2, _ := db.Has(key2)
		require.True(t, ok2)
	})

}
