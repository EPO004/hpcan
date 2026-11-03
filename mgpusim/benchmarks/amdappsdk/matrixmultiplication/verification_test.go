package matrixmultiplication

import "testing"

func TestVerifyChecksEveryOutputRow(t *testing.T) {
	b := &Benchmark{MatrixA: &Matrix{Width: 2, Height: 3, Data: []float32{1, 2, 3, 4, 5, 6}}, MatrixB: &Matrix{Width: 2, Height: 2, Data: []float32{1, 0, 0, 1}}}
	correct := (&CPUMatrixMultiplier{}).Multiply(b.MatrixA, b.MatrixB)
	b.MatrixC = &Matrix{Width: correct.Width, Height: correct.Height, Data: append([]float32(nil), correct.Data...)}
	b.Verify()
	b.MatrixC.Data[4] += 1 // Last row of a rectangular result; old verifier missed it.
	defer func() {
		if recover() == nil {
			t.Fatal("verification accepted corrupted output outside the first row")
		}
	}()
	b.Verify()
}
