package numerology

import (
	"database/sql"
	"encoding/json"
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
	LifePathNumber    int      `json:"life_path_number"`
	LuckyNumbers      []string `json:"lucky_numbers"`       // Lifetime
	WeeklyNumbers     []string `json:"weekly_lucky_numbers"` // This week
	DailyNumbers      []string `json:"daily_lucky_numbers"`  // Today
	PersonalWeek      int      `json:"personal_week"`
	PersonalDay       int      `json:"personal_day"`
	WeekLabel         string   `json:"week_label"`
	DayLabel          string   `json:"day_label"`
	ExplanationMM     string   `json:"explanation_mm"`
	ExplanationEN     string   `json:"explanation_en"`
	Birthdate         string   `json:"birthdate"`
	CalculatedAt      string   `json:"calculated_at"`
}

var db *sql.DB

// yangonLoc is the Yangon/Myanmar timezone (GMT+6:30)
var yangonLoc *time.Location

func init() {
	var err error
	yangonLoc, err = time.LoadLocation("Asia/Yangon")
	if err != nil {
		// Fallback: manual offset GMT+6:30
		yangonLoc = time.FixedZone("Asia/Yangon", 6*3600+30*60)
	}
}

// InitDB initializes the numerology module and creates cache table
func InitDB(database *sql.DB) error {
	db = database
	if db != nil {
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS numerology_cache (
				birthdate   TEXT NOT NULL,
				result_json TEXT NOT NULL,
				cached_date TEXT NOT NULL,
				updated_at  TEXT NOT NULL,
				PRIMARY KEY (birthdate, cached_date)
			)
		`)
		if err != nil {
			fmt.Println("⚠️  Numerology cache table error:", err)
		} else {
			fmt.Println("✅ Numerology cache table ready")
		}
	}
	fmt.Println("✅ Numerology module initialized (Yangon timezone)")
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

	// Current time in Yangon timezone
	nowYangon := time.Now().In(yangonLoc)

	// Calculate numerology
	result := calculateLuckyNumbers(birthdate, nowYangon)

	// Save to server cache (best-effort)
	if db != nil {
		if jsonBytes, err := json.Marshal(result); err == nil {
			saveCacheServer(req.Birthdate, nowYangon.Format("2006-01-02"), string(jsonBytes))
		}
	}

	c.JSON(http.StatusOK, result)
}

// calculateLuckyNumbers performs Myanmar numerology calculation
// nowYangon = current time already in Yangon timezone
func calculateLuckyNumbers(birthdate time.Time, nowYangon time.Time) NumerologyResult {
	year := birthdate.Year()
	month := int(birthdate.Month())
	day := birthdate.Day()

	// ── LIFETIME ──────────────────────────────────────────────────────
	lifePathNumber := calculateLifePath(year, month, day)
	dayMonthNumber := (day + month) % 100
	yearSum := sumDigits(year)
	yearNumber := (yearSum + day) % 100
	fullSum := sumDigits(year) + sumDigits(month) + sumDigits(day)
	fullNumber := fullSum % 100
	luckyNumbers := generateUniqueLuckyNumbers([]int{
		lifePathNumber, dayMonthNumber, yearNumber, fullNumber,
	})

	// ── WEEKLY (resets every Monday in Yangon time) ────────────────────
	_, weekNum := nowYangon.ISOWeek()             // ISO week 1-53
	weekdayNum := int(nowYangon.Weekday())         // 0=Sun
	if weekdayNum == 0 { weekdayNum = 7 }          // Sun=7
	personalWeek := reduceToSingle(lifePathNumber + weekNum)
	weeklyBase := (personalWeek*7 + weekNum) % 100
	weeklyNumbers := generateUniqueLuckyNumbers([]int{
		weeklyBase,
		(personalWeek + weekdayNum) % 100,
		(lifePathNumber + weekNum + weekdayNum) % 100,
		(weeklyBase + personalWeek) % 100,
	})
	// Week range: Monday → Friday only (lottery days Mon-Fri)
	weekday := int(nowYangon.Weekday()) // 0=Sun
	if weekday == 0 { weekday = 7 }
	monday := nowYangon.AddDate(0, 0, -(weekday - 1))
	friday := monday.AddDate(0, 0, 4)
	var weekLabel string
	if monday.Month() == friday.Month() {
		weekLabel = fmt.Sprintf("%s %d-%d (Mon-Fri)",
			monday.Format("Jan"), monday.Day(), friday.Day())
	} else {
		weekLabel = fmt.Sprintf("%d %s-%d %s (Mon-Fri)",
			monday.Day(), monday.Format("Jan"),
			friday.Day(), friday.Format("Jan"))
	}

	// ── DAILY (changes every day in Yangon time) ──────────────────────
	todayDay := nowYangon.Day()
	todayMonth := int(nowYangon.Month())
	personalDay := reduceToSingle(lifePathNumber + todayDay + todayMonth)
	dailyBase := (personalDay*3 + todayDay) % 100
	dailyNumbers := generateUniqueLuckyNumbers([]int{
		dailyBase,
		(personalDay + todayDay) % 100,
		(lifePathNumber + todayDay + todayMonth) % 100,
		(dailyBase + personalDay + todayMonth) % 100,
	})
	dayLabel := nowYangon.Format("02 Jan (Monday)")

	// ── EXPLANATIONS ──────────────────────────────────────────────────
	explanationMM := generateExplanationMM(lifePathNumber, len(luckyNumbers))
	explanationEN := generateExplanationEN(lifePathNumber, len(luckyNumbers))

	return NumerologyResult{
		LifePathNumber: lifePathNumber,
		LuckyNumbers:   luckyNumbers,
		WeeklyNumbers:  weeklyNumbers,
		DailyNumbers:   dailyNumbers,
		PersonalWeek:   personalWeek,
		PersonalDay:    personalDay,
		WeekLabel:      weekLabel,
		DayLabel:       dayLabel,
		ExplanationMM:  explanationMM,
		ExplanationEN:  explanationEN,
		Birthdate:      birthdate.Format("2006-01-02"),
		CalculatedAt:   nowYangon.Format("2006-01-02 15:04:05 MST"),
	}
}

// saveCacheServer saves result JSON to server SQLite cache (best-effort)
func saveCacheServer(birthdate, cachedDate, resultJSON string) {
	if db == nil { return }
	_, _ = db.Exec(`
		INSERT INTO numerology_cache (birthdate, result_json, cached_date, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (birthdate, cached_date) DO UPDATE
		SET result_json = EXCLUDED.result_json, updated_at = EXCLUDED.updated_at
	`, birthdate, resultJSON, cachedDate, time.Now().In(yangonLoc).Format(time.RFC3339))
}

// calculateLifePath calculates the life path number (1-9, or master numbers 11/22)
// Traditional method: reduce each component separately, then sum.
// If final sum (or intermediate) is 11 or 22, preserve as master number.
func calculateLifePath(year, month, day int) int {
	d := reduceComponent(day)
	m := reduceComponent(month)
	y := reduceComponent(year)
	sum := d + m + y
	// Check master number before final reduction
	if sum == 11 || sum == 22 {
		return sum
	}
	return reduceToSingle(sum)
}

// reduceComponent reduces a date component to single digit,
// but preserves 11 and 22 as master numbers.
func reduceComponent(num int) int {
	if num == 11 || num == 22 {
		return num
	}
	return reduceToSingle(num)
}

// reduceToSingle reduces any number to 1-9
func reduceToSingle(num int) int {
	for num > 9 {
		num = sumDigits(num)
	}
	if num == 0 { return 9 }
	return num
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
	var isMaster bool

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
		personality = "လူသားချင်းစာနာမှု၊ ကြင်နာမှု"
	case 11:
		personality = "နိဒါန်းပညာ၊ ဝိညာဉ်ရေးရာ အလင်းပေးမှု"
		isMaster = true
	case 22:
		personality = "ကြီးမြတ်သောတည်ဆောက်မှု၊ လောကတော်လှပ်ငြိမ်းမှု"
		isMaster = true
	default:
		personality = "ထူးခြားမှု၊ ကံကြမ္မာ"
	}

	if isMaster {
		return fmt.Sprintf("သင့်ရဲ့ ဘဝကံကြမ္မာဂဏန်းက မာစတာနံပါတ် %d ဖြစ်ပြီး %s ရှိပါတယ်။ ဒါဟာ အလွန်ထူးခြားတဲ့ ကံကြမ္မာပါ။ သင့်အတွက် ကံကောင်းတဲ့ နံပါတ် %d လုံးကို တွက်ချက်ပေးထားပါတယ်။",
			lifePathNumber, personality, count)
	}
	return fmt.Sprintf("သင့်ရဲ့ ဘဝကံကြမ္မာဂဏန်းက %d ဖြစ်ပြီး %s ရှိပါတယ်။ သင့်အတွက် ကံကောင်းတဲ့ နံပါတ် %d လုံးကို တွက်ချက်ပေးထားပါတယ်။",
		lifePathNumber, personality, count)
}

// generateExplanationEN generates English explanation
func generateExplanationEN(lifePathNumber, count int) string {
	var personality string
	var isMaster bool

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
	case 11:
		personality = "intuition, spiritual enlightenment and inspiration"
		isMaster = true
	case 22:
		personality = "master builder, turning dreams into reality"
		isMaster = true
	default:
		personality = "unique destiny"
	}

	if isMaster {
		return fmt.Sprintf("Your life path number is Master Number %d, representing %s. This is a rare and powerful destiny. We've calculated %d lucky numbers for you.",
			lifePathNumber, personality, count)
	}
	return fmt.Sprintf("Your life path number is %d, representing %s. We've calculated %d lucky numbers for you.",
		lifePathNumber, personality, count)
}

// FormatNumber formats a number as 2-digit string
func formatNumber(num int) string {
	return fmt.Sprintf("%02d", num%100)
}
