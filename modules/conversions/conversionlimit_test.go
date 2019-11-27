package conversions_test

import (
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"testing"

	. "github.com/pegnet/pegnet/modules/conversions"
	"github.com/pegnet/pegnet/modules/transactionid"
)

func TestPayout(t *testing.T) {
	type Vector struct {
		Requested      uint64
		TotalRequested uint64
		Bank           uint64
		Payout         uint64
	}

	vecs := []Vector{
		{0, 0, 0, 0},
		{2500, 5000, 5000, 2500},
		{2500, 50000, 5000, 250},
		{2500, 500000, 5000, 25},
		{2500, 5000000, 5000, 2},
		{2500, 50000000, 5000, 0},
	}

	for i, v := range vecs {
		if pay := Payout(v.Requested, v.Bank, v.TotalRequested); pay != v.Payout {
			t.Errorf("Vector %d has payout of %d, exp %d", i, pay, v.Payout)
		}
	}

}

func TestNewConversionSupply(t *testing.T) {
	testBank := func(bank uint64) {
		s := NewConversionSupply(bank)
		totalReq := new(big.Int)

		for i := 0; i < rand.Intn(100); i++ {
			eHash := fmt.Sprintf("%064d", i)
			amt := rand.Uint64()
			totalReq.Add(totalReq, new(big.Int).SetUint64(amt))
			err := s.AddConversion(transactionid.FormatTxID(0, eHash), amt)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
		}

		pays := s.Payouts()
		var totalPay uint64
		for _, a := range pays {
			totalPay += a
		}
		if totalPay > bank {
			t.Errorf("total paid %d, that is over the bank of %d", totalPay, bank)
		}

		if totalReq.IsUint64() && totalReq.Uint64() < bank {
			if totalPay != totalReq.Uint64() {
				t.Errorf("[under bank] exp %d total pay, found %d", totalReq.Uint64(), totalPay)
			}
		} else {
			if totalPay != bank {
				t.Errorf("[over bank] exp %d total pay, found %d", bank, totalPay)
			}
		}

		// Each time we call payouts, the order we go through is decided by
		// the map. So the order is different, but the output should be the
		// same.

		for i := 0; i < 10; i++ {
			pays2 := s.Payouts()
			if !reflect.DeepEqual(pays, pays2) {
				t.Error("payout is inconsistent")
			}
		}

	}

	t.Run("ensure payouts never exceed limit (5K)", func(t *testing.T) {
		for i := 0; i < 5000; i++ {
			testBank(PerBlock)
		}
	})

	t.Run("ensure payouts never exceed limit (Random)", func(t *testing.T) {
		for i := 0; i < 5000; i++ {
			// 10 Mil limit
			testBank(rand.Uint64() % (1e7 * 1e8))
		}
	})

	t.Run("ensure payouts never exceed limit (low bank)", func(t *testing.T) {
		for i := 0; i < 5000; i++ {
			// Low limits
			testBank(rand.Uint64() % PerBlock)
		}
	})
}

func TestNewConversionSupply_Errors(t *testing.T) {
	t.Run("duplicate txid", func(t *testing.T) {
		s := NewConversionSupply(PerBlock)
		_ = s.AddConversion("0-0000000000000000000000000000000000000000000000000000000000000000", 100e8)
		err := s.AddConversion("0-0000000000000000000000000000000000000000000000000000000000000000", 100e8)
		if err == nil || err.Error() != "txid already exists in the this set" {
			t.Error("Expected error")
		}
	})

	t.Run("bad txid", func(t *testing.T) {
		s := NewConversionSupply(PerBlock)
		err := s.AddConversion("0-00000000000000000000000000000000", 100e8)
		if err == nil || err.Error() != "hash must be 32 bytes (64 hex characters)" {
			t.Error("Expected error")
		}
	})
}

func TestNewConversionSupply_Simple(t *testing.T) {
	testBank := func(amt int, per uint64) *ConversionSupplySet {
		s := NewConversionSupply(PerBlock)
		totalReq := new(big.Int)

		for i := 0; i < amt; i++ {
			eHash := fmt.Sprintf("%064d", i)
			amt := per
			totalReq.Add(totalReq, new(big.Int).SetUint64(amt))
			err := s.AddConversion(transactionid.FormatTxID(0, eHash), amt)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
		}
		return s
	}

	checkExact := func(s *ConversionSupplySet, per uint64) *ConversionSupplySet {
		pays := s.Payouts()
		for _, amt := range pays {
			if amt != per {
				t.Errorf("got %d, exp %d", amt, per)
			}
		}
		return s
	}

	t.Run("bank is enough", func(t *testing.T) {
		// Under the bank
		checkExact(testBank(4, 1000e8), 1000e8)

		// On the bank
		checkExact(testBank(5, 1000e8), 1000e8)
	})

	t.Run("tied for most", func(t *testing.T) {
		s := testBank(3, 5000e8)
		pays := s.Payouts()
		if pays[tID(0)] <= pays[tID(1)] ||
			pays[tID(0)] <= pays[tID(2)] {
			t.Errorf("txid 0 should be a bit higher: %v", pays)
		}
	})
}

func tID(i int) string {
	return fmt.Sprintf("%d-%064d", i, 0)
}