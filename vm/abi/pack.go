package abi

import (
	"math/big"
	"reflect"

	"github.com/qlcchain/go-qlc/common/types"
	"github.com/qlcchain/go-qlc/common/util"
)

// packBytesSlice packs the given bytes as [L, V] as the canonical representation
// bytes slice
func packBytesSlice(bytes []byte, l int) []byte {
	len := packNum(reflect.ValueOf(l))
	return append(len, util.RightPadBytes(bytes, (l+util.WordSize-1)/util.WordSize*util.WordSize)...)
}

// packElement packs the given reflect value according to the abi specification in
// t.
func packElement(t Type, reflectValue reflect.Value) []byte {
	switch t.T {
	case IntTy, UintTy:
		return packNum(reflectValue)
	case StringTy:
		return packBytesSlice([]byte(reflectValue.String()), reflectValue.Len())
	case AddressTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}

		return util.LeftPadBytes(reflectValue.Bytes(), util.WordSize)
	case TokenIdTy, HashTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}

		return util.LeftPadBytes(reflectValue.Bytes(), util.WordSize)
	case SignatureTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}

		return util.LeftPadBytes(reflectValue.Bytes(), util.WordSize*2)
	case BalanceTy:
		v := reflectValue.Interface().(types.Balance).Bytes()
		l := len(v)
		r := packNum(reflect.ValueOf(len(v)))
		return append(r, util.LeftPadBytes(v, (l+util.WordSize-1)/util.WordSize*util.WordSize)...)
	case BoolTy:
		if reflectValue.Bool() {
			return util.PaddedBigBytes(util.Big1, util.WordSize)
		}
		return util.PaddedBigBytes(util.Big0, util.WordSize)
	case BytesTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}
		return packBytesSlice(reflectValue.Bytes(), reflectValue.Len())
	case FixedBytesTy, FunctionTy:
		if reflectValue.Kind() == reflect.Array {
			reflectValue = mustArrayToByteSlice(reflectValue)
		}
		return util.RightPadBytes(reflectValue.Bytes(), util.WordSize)
	default:
		panic("abi: fatal error")
	}
}

// packNum packs the given number (using the reflect value) and will cast it to appropriate number representation
func packNum(value reflect.Value) []byte {
	switch kind := value.Kind(); kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return U256(new(big.Int).SetUint64(value.Uint()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return U256(big.NewInt(value.Int()))
	case reflect.Ptr:
		return U256(value.Interface().(*big.Int))
	default:
		panic("abi: fatal error")
	}

}
