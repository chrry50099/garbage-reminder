//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"telegram-garbage-reminder/internal/gemini"
)

func main() {
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		log.Fatal("請設定 GEMINI_API_KEY 環境變數")
	}

	ctx := context.Background()
	geminiClient, err := gemini.NewGeminiClient(ctx, geminiAPIKey, "gemini-2.0-flash-exp")
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	// 測試關鍵地址
	testAddress := "台北市中正區重慶南路一段122號"
	
	fmt.Printf("🧪 測試地址: %s\n", testAddress)
	fmt.Println(strings.Repeat("=", 50))

	// 測試意圖分析
	fmt.Println("1️⃣ 測試意圖分析...")
	intent, err := geminiClient.AnalyzeIntent(ctx, testAddress)
	if err != nil {
		fmt.Printf("❌ 意圖分析失敗: %v\n", err)
	} else {
		fmt.Printf("✅ 意圖分析成功:\n")
		fmt.Printf("   District: '%s'\n", intent.District)
		fmt.Printf("   Keywords: %v\n", intent.Keywords)
		
		// 檢查是否正確提取了區域
		if intent.District == "台北市中正區" {
			fmt.Println("🎯 完美！正確提取了 '台北市中正區'")
		} else if intent.District == "台北市" {
			fmt.Println("⚠️ 只提取了 '台北市'，建議改善")
		} else {
			fmt.Printf("❌ 意外的結果: '%s'\n", intent.District)
		}
	}

	fmt.Println("\n2️⃣ 測試地址提取...")
	extractedLocation, err := geminiClient.ExtractLocationFromText(ctx, testAddress)
	if err != nil {
		fmt.Printf("❌ 地址提取失敗: %v\n", err)
	} else {
		fmt.Printf("✅ 地址提取成功: '%s'\n", extractedLocation)
		
		// 檢查是否包含完整地址
		if extractedLocation == testAddress {
			fmt.Println("🎯 完美！提取了完整地址")
		} else if extractedLocation != "" {
			fmt.Printf("⚠️ 提取了部分地址: '%s'\n", extractedLocation)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("測試完成！")
}
