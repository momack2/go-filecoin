package core

import (
	"sort"
	"testing"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mockSigner types.MockSigner

func init() {
	ki := types.MustGenerateKeyInfo(2)
	mockSigner = types.NewMockSigner(ki)
}

func block(require *require.Assertions, height int, parentCid *cid.Cid, parentWeight uint64, msg string) *types.Block {
	addrGetter := types.NewAddressForTestGetter()

	m1 := types.NewMessage(mockSigner.Addresses[0], addrGetter(), 0, types.NewAttoFILFromFIL(10), "hello", []byte(msg))
	sm1, err := types.NewSignedMessage(*m1, &mockSigner)
	require.NoError(err)
	ret := []byte{1, 2}

	return &types.Block{
		Parents:           types.NewSortedCidSet(parentCid),
		ParentWeightNum:   types.Uint64(parentWeight),
		ParentWeightDenom: types.Uint64(uint64(1)),
		Height:            types.Uint64(42 + uint64(height)),
		Nonce:             7,
		Messages:          []*types.SignedMessage{sm1},
		StateRoot:         types.SomeCid(),
		MessageReceipts:   []*types.MessageReceipt{{ExitCode: 1, Return: []types.Bytes{ret}}},
	}
}

func TestTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()

	b1 := block(require, 1, cid1, uint64(1137), "1")
	b2 := block(require, 1, cid1, uint64(1137), "2")
	b3 := block(require, 1, cid1, uint64(1137), "3")

	ts := TipSet{}
	ts[b1.Cid().String()] = b1

	ts2 := ts.Clone()
	assert.Equal(ts2, ts) // note: assert.Equal() does a deep comparison, not same as Golang == operator
	assert.False(&ts2 == &ts)

	ts[b2.Cid().String()] = b2
	assert.NotEqual(ts2, ts)
	assert.Equal(2, len(ts))
	assert.Equal(1, len(ts2))

	ts2 = ts.Clone()
	assert.Equal(ts2, ts)
	ts2[b1.Cid().String()] = b3
	assert.NotEqual(ts2, ts)
	assert.Equal([]byte("3"), ts2[b1.Cid().String()].Messages[0].Params)
	assert.Equal([]byte("1"), ts[b1.Cid().String()].Messages[0].Params)

	// The actual values inside the TipSets are not copied - we assume they are used immutably.
	ts2 = ts.Clone()
	assert.Equal(ts2, ts)
	oldB1 := ts[b1.Cid().String()]
	ts[oldB1.Cid().String()].Nonce = 17
	assert.Equal(ts2, ts)
}

// Test methods: String, ToSortedCidSet, ToSlice, MinTicket, Height, NewTipSet, Equals
func RequireTestBlocks(t *testing.T) (*types.Block, *types.Block, *types.Block) {
	require := require.New(t)

	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()
	pW := uint64(1337)

	b1 := block(require, 1, cid1, pW, "1")
	b1.Ticket = []byte{0}
	b2 := block(require, 1, cid1, pW, "2")
	b2.Ticket = []byte{1}
	b3 := block(require, 1, cid1, pW, "3")
	b3.Ticket = []byte{0}
	return b1, b2, b3
}

func RequireTestTipSet(t *testing.T) TipSet {
	require := require.New(t)
	b1, b2, b3 := RequireTestBlocks(t)
	return RequireNewTipSet(require, b1, b2, b3)
}

func TestTipSetAddBlock(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	b1, b2, b3 := RequireTestBlocks(t)

	// Add Valid
	ts1 := TipSet{}
	RequireTipSetAdd(require, b1, ts1)
	assert.Equal(1, len(ts1))
	RequireTipSetAdd(require, b2, ts1)
	RequireTipSetAdd(require, b3, ts1)

	ts2 := RequireNewTipSet(require, b1, b2, b3)
	assert.Equal(ts2, ts1)

	// Invalid height
	b2.Height = 5
	ts := TipSet{}
	RequireTipSetAdd(require, b1, ts)
	err := ts.AddBlock(b2)
	assert.EqualError(err, ErrBadTipSetAdd.Error())
	b2.Height = b1.Height

	// Invalid parent set
	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()
	cid2 := cidGetter()
	b2.Parents = types.NewSortedCidSet(cid1, cid2)
	ts = TipSet{}
	RequireTipSetAdd(require, b1, ts)
	err = ts.AddBlock(b2)
	assert.EqualError(err, ErrBadTipSetAdd.Error())
	b2.Parents = b1.Parents

	// Invalid weight
	b2.ParentWeightNum = types.Uint64(3)
	ts = TipSet{}
	RequireTipSetAdd(require, b1, ts)
	err = ts.AddBlock(b2)
	assert.EqualError(err, ErrBadTipSetAdd.Error())
}

func TestNewTipSet(t *testing.T) {
	assert := assert.New(t)
	b1, b2, b3 := RequireTestBlocks(t)

	// Valid blocks
	ts, err := NewTipSet(b1, b2, b3)
	assert.NoError(err)
	assert.Equal(ts[b1.Cid().String()], b1)
	assert.Equal(ts[b2.Cid().String()], b2)
	assert.Equal(ts[b3.Cid().String()], b3)
	assert.Equal(3, len(ts))

	// Invalid heights
	b1.Height = 3
	ts, err = NewTipSet(b1, b2, b3)
	assert.EqualError(err, ErrBadTipSetCreate.Error())
	assert.Nil(ts)
	b1.Height = b2.Height

	// Invalid parent sets
	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()
	cid2 := cidGetter()
	b1.Parents = types.NewSortedCidSet(cid1, cid2)
	ts, err = NewTipSet(b1, b2, b3)
	assert.EqualError(err, ErrBadTipSetCreate.Error())
	assert.Nil(ts)
	b1.Parents = b2.Parents

	// Invalid parent weights
	b1.ParentWeightNum = types.Uint64(3)
	ts, err = NewTipSet(b1, b2, b3)
	assert.EqualError(err, ErrBadTipSetCreate.Error())
	assert.Nil(ts)
}

func TestTipSetMinTicket(t *testing.T) {
	assert := assert.New(t)
	ts := RequireTestTipSet(t)
	mt, err := ts.MinTicket()
	assert.NoError(err)
	assert.Equal(types.Signature([]byte{0}), mt)
}

func TestTipSetHeight(t *testing.T) {
	assert := assert.New(t)
	ts := RequireTestTipSet(t)
	h, err := ts.Height()
	assert.NoError(err)
	assert.Equal(uint64(43), h)
}

func TestTipSetParents(t *testing.T) {
	assert := assert.New(t)
	b1, _, _ := RequireTestBlocks(t)
	ts := RequireTestTipSet(t)
	ps, err := ts.Parents()
	assert.NoError(err)
	assert.Equal(ps, b1.Parents)
}

func TestTipSetParentWeight(t *testing.T) {
	assert := assert.New(t)
	ts := RequireTestTipSet(t)
	wNum, wDenom, err := ts.ParentWeight()
	assert.NoError(err)
	assert.Equal(wNum, uint64(1337))
	assert.Equal(wDenom, uint64(1))
}

func TestTipSetToSortedCidSet(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	assert := assert.New(t)

	cidsExp := types.NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid())
	assert.Equal(cidsExp, ts.ToSortedCidSet())
}

func TestTipSetString(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	assert := assert.New(t)

	cidsExp := types.NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid())
	strExp := cidsExp.String()
	assert.Equal(strExp, ts.String())
}

func TestTipSetToSlice(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	tips := []*types.Block{b1, b2, b3}
	assert := assert.New(t)

	blks := ts.ToSlice()
	sort.Slice(tips, func(i, j int) bool {
		return tips[i].Cid().String() < tips[j].Cid().String()
	})
	sort.Slice(blks, func(i, j int) bool {
		return blks[i].Cid().String() < blks[j].Cid().String()
	})
	assert.Equal(tips, blks)

	tsEmpty := TipSet{}
	slEmpty := tsEmpty.ToSlice()
	assert.Equal(0, len(slEmpty))
}

func TestTipSetEquals(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	assert := assert.New(t)
	require := require.New(t)

	ts2 := RequireNewTipSet(require, b1, b2)
	assert.True(!ts2.Equals(ts))
	ts2.AddBlock(b3)
	assert.True(ts.Equals(ts2))
}

func TestTipIndex(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	idx := tipIndex{}

	contains := func(b *types.Block, expectedHeightEntries, expectedParentSetEntries, expectedBlocks int) {
		assert.Equal(expectedHeightEntries, len(idx))
		assert.Equal(expectedParentSetEntries, len(idx[uint64(b.Height)]))
		assert.Equal(expectedBlocks, len(idx[uint64(b.Height)][keyForParentSet(b.Parents)]))
		assert.True(b.Cid().Equals(idx[uint64(b.Height)][keyForParentSet(b.Parents)][b.Cid().String()].Cid()))
	}

	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()
	b1 := block(require, 42, cid1, uint64(1137), "foo")
	idx.addBlock(b1)
	contains(b1, 1, 1, 1)

	b2 := block(require, 42, cid1, uint64(1137), "bar")
	idx.addBlock(b2)
	contains(b2, 1, 1, 2)

	cid3 := cidGetter()
	b3 := block(require, 42, cid3, uint64(1137), "hot")
	idx.addBlock(b3)
	contains(b3, 1, 2, 1)

	cid4 := cidGetter()
	b4 := block(require, 43, cid4, uint64(1137), "monkey")
	idx.addBlock(b4)
	contains(b4, 2, 1, 1)
}
