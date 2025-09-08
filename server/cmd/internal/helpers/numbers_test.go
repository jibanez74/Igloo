package helpers

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestGetPreciseDecimalFromStr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // Expected decimal value as string for comparison
		wantErr  bool
	}{
		{
			name:     "ffprobe duration with 6 decimal places",
			input:    "294.870204",
			expected: "294.870", // Should round to 3 decimal places
			wantErr:  false,
		},
		{
			name:     "ffprobe duration with 3 decimal places",
			input:    "123.456",
			expected: "123.456",
			wantErr:  false,
		},
		{
			name:     "ffprobe duration with 2 decimal places",
			input:    "45.67",
			expected: "45.670", // Should pad to 3 decimal places
			wantErr:  false,
		},
		{
			name:     "ffprobe duration with 1 decimal place",
			input:    "30.5",
			expected: "30.500",
			wantErr:  false,
		},
		{
			name:     "ffprobe duration whole number",
			input:    "180",
			expected: "180.000",
			wantErr:  false,
		},
		{
			name:     "ffprobe duration with many decimal places",
			input:    "123.456789012",
			expected: "123.457", // Should round to 3 decimal places
			wantErr:  false,
		},
		{
			name:     "ffprobe duration with trailing zeros",
			input:    "99.100000",
			expected: "99.100",
			wantErr:  false,
		},
		{
			name:     "ffprobe duration very small",
			input:    "0.001",
			expected: "0.001",
			wantErr:  false,
		},
		{
			name:     "ffprobe duration very small with rounding",
			input:    "0.0005",
			expected: "0.001", // Should round up
			wantErr:  false,
		},
		{
			name:     "ffprobe duration zero",
			input:    "0",
			expected: "0.000",
			wantErr:  false,
		},
		{
			name:     "ffprobe duration negative (should not happen but test edge case)",
			input:    "-1.234",
			expected: "-1.234",
			wantErr:  false,
		},
		{
			name:     "invalid string",
			input:    "not_a_number",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "string with spaces",
			input:    " 123.456 ",
			expected: "123.456",
			wantErr:  false,
		},
		{
			name:     "scientific notation",
			input:    "1.23e2",
			expected: "123.000",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetPreciseDecimalFromStr(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetPreciseDecimalFromStr() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("GetPreciseDecimalFromStr() unexpected error: %v", err)
				return
			}

			// Check if the result is valid
			if !result.Valid {
				t.Errorf("GetPreciseDecimalFromStr() result should be valid")
				return
			}

			// Convert the result back to decimal for comparison
			// pgtype.Numeric stores the value in Int and Exp fields
			var resultDecimal decimal.Decimal
			if result.Int != nil {
				resultDecimal = decimal.NewFromBigInt(result.Int, result.Exp)
			} else {
				resultDecimal = decimal.Zero
			}

			// Compare with expected value
			expectedDecimal, err := decimal.NewFromString(tt.expected)
			if err != nil {
				t.Errorf("Failed to parse expected value: %v", err)
				return
			}

			if !resultDecimal.Equal(expectedDecimal) {
				t.Errorf("GetPreciseDecimalFromStr() = %v, expected %v", resultDecimal.String(), tt.expected)
			}
		})
	}
}

func TestGetPreciseDecimalFromStr_EdgeCases(t *testing.T) {
	t.Run("very large number", func(t *testing.T) {
		// Test with a number that might exceed NUMERIC(10,3) precision
		result, err := GetPreciseDecimalFromStr("9999999.999")
		if err != nil {
			t.Errorf("GetPreciseDecimalFromStr() unexpected error: %v", err)
			return
		}

		if !result.Valid {
			t.Errorf("GetPreciseDecimalFromStr() result should be valid")
		}

		// The result should be rounded to 3 decimal places
		var resultDecimal decimal.Decimal
		if result.Int != nil {
			resultDecimal = decimal.NewFromBigInt(result.Int, result.Exp)
		} else {
			resultDecimal = decimal.Zero
		}

		expected := "9999999.999"
		expectedDecimal, _ := decimal.NewFromString(expected)
		if !resultDecimal.Equal(expectedDecimal) {
			t.Errorf("GetPreciseDecimalFromStr() = %v, expected %v", resultDecimal.String(), expected)
		}
	})

	t.Run("rounding behavior", func(t *testing.T) {
		// Test specific rounding cases
		roundingTests := []struct {
			input    string
			expected string
		}{
			{"0.0004", "0.000"}, // Should round down
			{"0.0005", "0.001"}, // Should round up
			{"0.0006", "0.001"}, // Should round up
			{"1.2345", "1.235"}, // Should round up
			{"1.2344", "1.234"}, // Should round down
		}

		for _, rt := range roundingTests {
			result, err := GetPreciseDecimalFromStr(rt.input)
			if err != nil {
				t.Errorf("GetPreciseDecimalFromStr(%s) unexpected error: %v", rt.input, err)
				continue
			}

			var resultDecimal decimal.Decimal
			if result.Int != nil {
				resultDecimal = decimal.NewFromBigInt(result.Int, result.Exp)
			} else {
				resultDecimal = decimal.Zero
			}

			expectedDecimal, _ := decimal.NewFromString(rt.expected)
			if !resultDecimal.Equal(expectedDecimal) {
				t.Errorf("GetPreciseDecimalFromStr(%s) = %v, expected %v", rt.input, resultDecimal.String(), rt.expected)
			}
		}
	})
}

func TestGetPreciseDecimalFromStr_RealFfprobeData(t *testing.T) {
	// Test with real ffprobe duration data from the example
	realDuration := "294.870204"
	result, err := GetPreciseDecimalFromStr(realDuration)
	if err != nil {
		t.Errorf("GetPreciseDecimalFromStr() unexpected error: %v", err)
		return
	}

	if !result.Valid {
		t.Errorf("GetPreciseDecimalFromStr() result should be valid")
		return
	}

	// Convert back to decimal for verification
	var resultDecimal decimal.Decimal
	if result.Int != nil {
		resultDecimal = decimal.NewFromBigInt(result.Int, result.Exp)
	} else {
		resultDecimal = decimal.Zero
	}

	// Should be rounded to 3 decimal places: 294.870
	expected := "294.870"
	expectedDecimal, _ := decimal.NewFromString(expected)
	if !resultDecimal.Equal(expectedDecimal) {
		t.Errorf("GetPreciseDecimalFromStr() = %v, expected %v", resultDecimal.String(), expected)
	}

	// Verify it can be stored in NUMERIC(10,3) format
	if resultDecimal.GreaterThan(decimal.NewFromInt(9999999)) {
		t.Errorf("Result exceeds NUMERIC(10,3) precision limit")
	}
}
