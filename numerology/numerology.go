package numerology
package numerology

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CalculateRequest represents the birthdate input
type CalculateRequest struct {
	Birthdate string `json:"birthdate" binding:"required"` // Format: YYYY-MM-DD
}

// NumerologyResult represents the calculated lucky numbers
type NumerologyResult struct {
	LifePathNumber int      `json:"life_path_number"`
	LuckyNumbers   []string `json:"lucky_numbers"`
	ExplanationMM  string   `json:"explanation_mm"`
	ExplanationEN  string   `json:"explanation_en"`
	Birthdate      string   `json:"birthdate"`
	CalculatedAt   string   `json:"calculated_at"`
}

var db *sql.DB

// InitDB initializes the numerology module (no database needed - pure calculation)
func InitDB(database *sql.DB) error {
	db = database
	fmt.Println("✅ Numerology module initialized (calculation-based)")
	return nil
}

// CalculateNumerology handles POST /api/game/numerology/calculate
func CalculateNumerology(c *gin.Context) {
	var req CalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. Format: YYYY-MM-DD"})
		return
	}

	// Parse birthdate
	birthdate, err := time.Parse("2006-01-02", req.Birthdate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}

	// Calculate numerology
	result := calculateLuckyNumbers(birthdate)
	
	c.JSON(http.StatusOK, result)
}

// calculateLuckyNumbers performs Myanmar numerology calculation
func calculateLuckyNumbers(birthdate time.Time) NumerologyResult {
	year := birthdate.Year()
	month := int(birthdate.Month())
	day := birthdate.Day()

	// Method 1: Life Path Number (sum all digits)
	lifePathNumber := calculateLifePath(year, month, day)

	// Method 2: Day + Month pattern
	dayMonthNumber := (day + month) % 100

	// Method 3: Year reduction
	yearSum := sumDigits(year)
	yearNumber := (yearSum + day) % 100

	// Method 4: Full date sum
	fullSum := sumDigits(year) + sumDigits(month) + sumDigits(day)
	fullNumber := fullSum % 100

	// Generate 4 unique lucky numbers
	luckyNumbers := generateUniqueLuckyNumbers([]int{
		lifePathNumber,
		dayMonthNumber,
		yearNumber,
		fullNumber,
	})

	// Generate explanations
	explanationMM := generateExplanationMM(lifePathNumber, len(luckyNumbers))
	explanationEN := generateExplanationEN(lifePathNumber, len(luckyNumbers))

	return NumerologyResult{
		LifePathNumber: lifePathNumber,
		LuckyNumbers:   luckyNumbers,
		ExplanationMM:  explanationMM,
		ExplanationEN:  explanationEN,
		Birthdate:      birthdate.Format("2006-01-02"),
		CalculatedAt:   time.Now().Format(time.RFC3339),
	}
}

// calculateLifePath calculates the life path number (1-9)
func calculateLifePath(year, month, day int) int {
	sum := sumDigits(year) + sumDigits(month) + sumDigits(day)
	
	// Reduce to single digit (1-9)
	for sum > 9 && sum != 11 && sum != 22 && sum != 33 {
		sum = sumDigits(sum)
	}
	
	// Master numbers stay as is, others reduce
	if sum == 11 || sum == 22 || sum == 33 {
		return sum % 10 // Convert master numbers to single digit for Myanmar style
	}
	
	return sum
}

// sumDigits sums all digits in a number
func sumDigits(num int) int {
	sum := 0
	for num > 0 {
		sum += num % 10
		num /= 10
	}
	return sum
}

// generateUniqueLuckyNumbers generates 3-4 unique 2-digit lucky numbers
func generateUniqueLuckyNumbers(candidates []int) []string {
	seen := make(map[int]bool)
	var numbers []string

	for _, num := range candidates {
		// Ensure 2-digit number (0-99)
		num = num % 100
		
		if !seen[num] {
			seen[num] = true
			numbers = append(numbers, fmt.Sprintf("%02d", num))
		}

		// Stop at 4 unique numbers
		if len(numbers) >= 4 {
			break
		}
	}

	// If we don't have 4 numbers, generate more using patterns
	if len(numbers) < 4 {
		baseNum := candidates[0] % 100
		for i := 1; len(numbers) < 4; i++ {
			derivedNum := (baseNum + (i * 11)) % 100
			if !seen[derivedNum] {
				seen[derivedNum] = true
				numbers = append(numbers, fmt.Sprintf("%02d", derivedNum))
			}
		}
	}

	return numbers
}

// generateExplanationMM generates Myanmar explanation
func generateExplanationMM(lifePathNumber, count int) string {
	var personality string
	
	switch lifePathNumber {
	case 1:
		personality = "ခေါင်းဆောင်မှု၊ လွတ်လပ်မှု"
	case 2:
		personality = "သဟဇာတ၊ ညီညွတ်မှု"
	case 3:
		personality = "ဖန်တီးမှု၊ ပျော်ရွှင်မှု"
	case 4:
		personality = "တည်ငြိမ်မှု၊ ကြိုးစားအားထုတ်မှု"
	case 5:
		personality = "လွတ်လပ်စီးပွားရေး၊ စွန့်စားမှု"
	case 6:
		personality = "တာဝန်သိမှု၊ မေတ္တာ"
	case 7:
		personality = "ဉာဏ်ပညာ၊ ဝိညာဉ်ရေးရာ"
	case 8:
		personality = "အောင်မြင်မှု၊ ကံကောင်းမှု"
	case 9:
		personality = "လူသားချင်းစာနာမှု၊ အတ္တဖြစ်မှု"
	default:
		personality = "ထူးခြားမှု၊ ကံကြမ္မာ"
	}

	return fmt.Sprintf("သင့်ရဲ့ ဘဝကံကြမ္မာဂဏန်းက %d ဖြစ်ပြီး %s ရှိပါတယ်။ သင့်အတွက် ကံကောင်းတဲ့ နံပါတ် %d လုံးကို တွက်ချက်ပေးထားပါတယ်။", 
		lifePathNumber, personality, count)
}

// generateExplanationEN generates English explanation
func generateExplanationEN(lifePathNumber, count int) string {
	var personality string
	
	switch lifePathNumber {
	case 1:
		personality = "leadership and independence"
	case 2:
		personality = "harmony and cooperation"
	case 3:
		personality = "creativity and joy"
	case 4:
		personality = "stability and hard work"
	case 5:
		personality = "freedom and adventure"
	case 6:
		personality = "responsibility and love"
	case 7:
		personality = "wisdom and spirituality"
	case 8:
		personality = "success and prosperity"
	case 9:
		personality = "compassion and idealism"
	default:
		personality = "unique destiny"
	}

	return fmt.Sprintf("Your life path number is %d, representing %s. We've calculated %d lucky numbers for you.", 
		lifePathNumber, personality, count)
}

// FormatNumber formats a number as 2-digit string
func formatNumber(num int) string {
	return fmt.Sprintf("%02d", num%100)
}
