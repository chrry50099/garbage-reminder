//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"telegram-garbage-reminder/internal/gemini"
)

func main() {
	// 從環境變數獲取 Gemini API key
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	
	if geminiAPIKey == "" {
		log.Fatal("請設定 GEMINI_API_KEY 環境變數")
	}

	ctx := context.Background()

	// 初始化 Gemini 客戶端
	geminiClient, err := gemini.NewGeminiClient(ctx, geminiAPIKey, "gemini-2.0-flash-exp")
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	// 測試地址
	testAddresses := []string{
		"台北市中正區重慶南路一段122號",
		"台北市大安區忠孝東路四段",
		"新北市板橋區縣民大道二段7號",
		"高雄市左營區博愛二路777號",
		"家",
		"公司",
		"我晚上七點前在台北市大安區哪裡倒垃圾？",
		"晚上六點半在哪裡倒垃圾？",
	}

	fmt.Println("開始測試 Gemini 地址處理邏輯...")
	fmt.Println(strings.Repeat("=", 60))

	for i, address := range testAddresses {
		fmt.Printf("\n測試 %d: %s\n", i+1, address)
		fmt.Println(strings.Repeat("-", 40))
		
		testGeminiProcessing(ctx, geminiClient, address)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("測試完成")
}

func testGeminiProcessing(ctx context.Context, geminiClient *gemini.GeminiClient, text string) {
	fmt.Printf("📍 原始輸入: %s\n", text)

	// Step 1: Gemini 意圖分析
	fmt.Println("\n1️⃣ Gemini 意圖分析...")
	intent, err := geminiClient.AnalyzeIntent(ctx, text)
	if err != nil {
		fmt.Printf("   ❌ 意圖分析失敗: %v\n", err)
	} else {
		fmt.Printf("   ✅ 意圖分析成功:\n")
		fmt.Printf("      District: '%s'\n", intent.District)
		fmt.Printf("      TimeWindow: From='%s', To='%s'\n", intent.TimeWindow.From, intent.TimeWindow.To)
		fmt.Printf("      Keywords: %v\n", intent.Keywords)
		fmt.Printf("      QueryType: '%s'\n", intent.QueryType)
	}

	// Step 2: Gemini 地址提取
	fmt.Println("\n2️⃣ Gemini 地址提取...")
	extractedLocation, err := geminiClient.ExtractLocationFromText(ctx, text)
	if err != nil {
		fmt.Printf("   ❌ 地址提取失敗: %v\n", err)
	} else {
		fmt.Printf("   ✅ 地址提取成功: '%s'\n", extractedLocation)
	}

	// Step 3: 本地地址簡化
	fmt.Println("\n3️⃣ 本地地址簡化...")
	simplifiedAddress := extractSimplifiedAddress(text)
	if simplifiedAddress != "" {
		fmt.Printf("   ✅ 簡化地址: '%s'\n", simplifiedAddress)
	} else {
		fmt.Printf("   ⚠️ 無法簡化地址\n")
	}

	// Step 4: 決定最終使用的地址
	fmt.Println("\n4️⃣ 最終地址選擇...")
	var finalAddress string
	var method string

	if intent != nil && intent.District != "" {
		finalAddress = intent.District
		method = "intent.District"
	} else if extractedLocation != "" && strings.TrimSpace(extractedLocation) != "" {
		finalAddress = strings.TrimSpace(extractedLocation)
		method = "gemini.ExtractLocation"
	} else {
		finalAddress = text
		method = "original.text"
	}

	fmt.Printf("   🎯 最終地址: '%s' (方法: %s)\n", finalAddress, method)

	// 如果最終地址失敗，會嘗試的 fallback
	fmt.Println("\n5️⃣ Fallback 策略:")
	if method != "original.text" {
		fmt.Printf("   📌 Fallback 1: 原始文字 '%s'\n", text)
	}
	if simplifiedAddress != "" && simplifiedAddress != finalAddress && simplifiedAddress != text {
		fmt.Printf("   📌 Fallback 2: 簡化地址 '%s'\n", simplifiedAddress)
	}
}

func extractSimplifiedAddress(text string) string {
	// 嘗試提取縣市區的模式
	patterns := []string{
		`(台北市|新北市|桃園市|台中市|台南市|高雄市|基隆市|新竹市|嘉義市)[^市]*?(區|市)`,
		`(新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|台東縣|澎湖縣|金門縣|連江縣)[^縣]*?(鄉|鎮|市)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(text); match != "" {
			return match
		}
	}
	
	// 如果沒有匹配，嘗試提取縣市
	cityPattern := `(台北市|新北市|桃園市|台中市|台南市|高雄市|基隆市|新竹市|嘉義市|新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|台東縣|澎湖縣|金門縣|連江縣)`
	re := regexp.MustCompile(cityPattern)
	if match := re.FindString(text); match != "" {
		return match
	}
	
	return ""
}
