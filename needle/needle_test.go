package needle

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	hiEntPayload, _ := hex.DecodeString("40e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e137b4beca9d4a963364612518b769b7982c36cd4b5c8e2e4eb97d1dd25ad07e8bf34bccf14107d66477e045e258abec451c1256e9d81a7099ede5a172d8ead79a09653accacebeb0976a74036a6ea20df8df266db4c4feb7d14de7a83e1254728a83dc0cc72c54610526673cc4e652f91b12bd2b6bae475e06e7268203470517bba6f4b0d76a71b3dcff675cced0c7d05d17228963d27db1d1ea5d2cadd59ff2fba5d10e2f42ca81e25021dc0cec2f50b91ac3382cde083cbdf439a65d8b65ee19dc17b67c30914fafca2137440206640816cd3e4f15aafd81d414dfe25119a77355490655f4783019fdf231dd5c7407ada6fd9386785a935cdb9a203e12d7546b16b86dbe74f4d2b5a171e69bb2fd36ae6670c6c4e1401ceef97aca38b7070f7")
	hiEntExpected, _ := hex.DecodeString("d5f335a2c6526e3f56fb38ada975b59e691382574d8c4bd5a10723bec3953d2440e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e137b4beca9d4a963364612518b769b7982c36cd4b5c8e2e4eb97d1dd25ad07e8bf34bccf14107d66477e045e258abec451c1256e9d81a7099ede5a172d8ead79a09653accacebeb0976a74036a6ea20df8df266db4c4feb7d14de7a83e1254728a83dc0cc72c54610526673cc4e652f91b12bd2b6bae475e06e7268203470517bba6f4b0d76a71b3dcff675cced0c7d05d17228963d27db1d1ea5d2cadd59ff2fba5d10e2f42ca81e25021dc0cec2f50b91ac3382cde083cbdf439a65d8b65ee19dc17b67c30914fafca2137440206640816cd3e4f15aafd81d414dfe25119a77355490655f4783019fdf231dd5c7407ada6fd9386785a935cdb9a203e12d7546b16b86dbe74f4d2b5a171e69bb2fd36ae6670c6c4e1401ceef97aca38b7070f7")

	testTable := []struct {
		payload     []byte
		expected    []byte
		hasError    bool
		description string
	}{
		{
			payload:     make([]byte, PayloadLength-1),
			expected:    nil,
			hasError:    true,
			description: "too few bytes, one less than expected",
		},
		{
			payload:     make([]byte, 0),
			expected:    nil,
			hasError:    true,
			description: "too few bytes, no bytes",
		},
		{
			payload:     make([]byte, PayloadLength+1),
			expected:    nil,
			hasError:    true,
			description: "too many bytes",
		},
		{
			payload:     make([]byte, 448),
			expected:    nil,
			hasError:    true,
			description: "low entropy payload",
		},
		{
			payload:     hiEntPayload,
			expected:    hiEntExpected,
			hasError:    false,
			description: "high entropy payload",
		},
	}

	for _, test := range testTable {
		n, err := New(test.payload)
		if err != nil {
			if !test.hasError {
				t.Errorf("test: %v had error: %v", test.description, err)
			}
		} else if !bytes.Equal(n.Bytes(), test.expected) {
			t.Errorf("%v, bytes not equal\n%x\n%x", test.description, n.Bytes(), test.expected)
		}
	}
}

func TestFromBytes(t *testing.T) {
	t.Parallel()

	validRaw, _ := hex.DecodeString("d5f335a2c6526e3f56fb38ada975b59e691382574d8c4bd5a10723bec3953d2440e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e137b4beca9d4a963364612518b769b7982c36cd4b5c8e2e4eb97d1dd25ad07e8bf34bccf14107d66477e045e258abec451c1256e9d81a7099ede5a172d8ead79a09653accacebeb0976a74036a6ea20df8df266db4c4feb7d14de7a83e1254728a83dc0cc72c54610526673cc4e652f91b12bd2b6bae475e06e7268203470517bba6f4b0d76a71b3dcff675cced0c7d05d17228963d27db1d1ea5d2cadd59ff2fba5d10e2f42ca81e25021dc0cec2f50b91ac3382cde083cbdf439a65d8b65ee19dc17b67c30914fafca2137440206640816cd3e4f15aafd81d414dfe25119a77355490655f4783019fdf231dd5c7407ada6fd9386785a935cdb9a203e12d7546b16b86dbe74f4d2b5a171e69bb2fd36ae6670c6c4e1401ceef97aca38b7070f7")
	validExpected, _ := hex.DecodeString("d5f335a2c6526e3f56fb38ada975b59e691382574d8c4bd5a10723bec3953d2440e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e137b4beca9d4a963364612518b769b7982c36cd4b5c8e2e4eb97d1dd25ad07e8bf34bccf14107d66477e045e258abec451c1256e9d81a7099ede5a172d8ead79a09653accacebeb0976a74036a6ea20df8df266db4c4feb7d14de7a83e1254728a83dc0cc72c54610526673cc4e652f91b12bd2b6bae475e06e7268203470517bba6f4b0d76a71b3dcff675cced0c7d05d17228963d27db1d1ea5d2cadd59ff2fba5d10e2f42ca81e25021dc0cec2f50b91ac3382cde083cbdf439a65d8b65ee19dc17b67c30914fafca2137440206640816cd3e4f15aafd81d414dfe25119a77355490655f4783019fdf231dd5c7407ada6fd9386785a935cdb9a203e12d7546b16b86dbe74f4d2b5a171e69bb2fd36ae6670c6c4e1401ceef97aca38b7070f7")
	invalidHash, _ := hex.DecodeString("11f335a2c6526e3f56fb38ada975b59e691382574d8c4bd5a10723bec3953d2440e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e137b4beca9d4a963364612518b769b7982c36cd4b5c8e2e4eb97d1dd25ad07e8bf34bccf14107d66477e045e258abec451c1256e9d81a7099ede5a172d8ead79a09653accacebeb0976a74036a6ea20df8df266db4c4feb7d14de7a83e1254728a83dc0cc72c54610526673cc4e652f91b12bd2b6bae475e06e7268203470517bba6f4b0d76a71b3dcff675cced0c7d05d17228963d27db1d1ea5d2cadd59ff2fba5d10e2f42ca81e25021dc0cec2f50b91ac3382cde083cbdf439a65d8b65ee19dc17b67c30914fafca2137440206640816cd3e4f15aafd81d414dfe25119a77355490655f4783019fdf231dd5c7407ada6fd9386785a935cdb9a203e12d7546b16b86dbe74f4d2b5a171e69bb2fd36ae6670c6c4e1401ceef97aca38b7070f7")

	testTable := []struct {
		rawBytes    []byte
		expected    []byte
		hasError    bool
		description string
	}{
		{
			rawBytes:    validRaw,
			expected:    validExpected,
			hasError:    false,
			description: "valid raw bytes",
		},
		{
			rawBytes:    make([]byte, 0),
			expected:    nil,
			hasError:    true,
			description: "empty bytes",
		},
		{
			rawBytes:    make([]byte, NeedleLength-1),
			expected:    nil,
			hasError:    true,
			description: "too few bytes, one less than expected",
		},
		{
			rawBytes:    make([]byte, 0),
			expected:    nil,
			hasError:    true,
			description: "too few bytes, no bytes",
		},
		{
			rawBytes:    make([]byte, NeedleLength+1),
			expected:    nil,
			hasError:    true,
			description: "too many bytes",
		},
		{
			rawBytes:    invalidHash,
			expected:    nil,
			hasError:    true,
			description: "invalid hash",
		},
	}
	for _, test := range testTable {
		n, err := FromBytes(test.rawBytes)
		if err != nil {
			if !test.hasError {
				t.Errorf("test: %v had error: %v", test.description, err)
			}
		} else if !bytes.Equal(n.Bytes(), test.expected) {
			t.Errorf("%v, bytes not equal\n%x\n%x", test.description, n.Bytes(), test.expected)
		}
	}
}
