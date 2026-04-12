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
	
	testCases := []struct{
		name string
		model string
	}{
		{"Production (gemini-2.0-flash)", "gemini-2.0-flash"},
		{"Experimental (gemini-2.0-flash-exp)", "gemini-2.0-flash-exp"},
	}
	
	testAddress := "新北市三重區仁義街"
	
	for _, tc := range testCases {
		fmt.Printf("\n🧪 測試 Model: %s\n", tc.name)
		fmt.Println(strings.Repeat("=", 60))
		
		geminiClient, err := gemini.NewGeminiClient(ctx, geminiAPIKey, tc.model)
		if err != nil {
			log.Printf("Failed to create Gemini client: %v", err)
			continue
		}
		
		fmt.Printf("測試地址: %s\n\n", testAddress)
		
		intent, err := geminiClient.AnalyzeIntent(ctx, testAddress)
		if err != nil {
			fmt.Printf("❌ 錯誤: %v\n", err)
		} else {
			fmt.Printf("District: '%s'\n", intent.District)
			fmt.Printf("Keywords: %v\n", intent.Keywords)
			
			if intent.District == "新北市三重區" {
				fmt.Println("✅ 完美！正確提取了 '新北市三重區'")
			} else if intent.District == "新北市" {
				fmt.Println("❌ 失敗：只提取了 '新北市'，應該是 '新北市三重區'")
			}
		}
		
		geminiClient.Close()
	}
	
	fmt.Println(strings.Repeat("=", 60))
}
