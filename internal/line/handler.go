package line

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"

	"telegram-garbage-reminder/internal/garbage"
	"telegram-garbage-reminder/internal/gemini"
	"telegram-garbage-reminder/internal/geo"
	"telegram-garbage-reminder/internal/store"
)

type Handler struct {
	messagingAPI    *messaging_api.MessagingApiAPI
	store           *store.FirestoreClient
	geoClient       *geo.GeocodeClient
	garbageAdapter  *garbage.GarbageAdapter
	geminiClient    *gemini.GeminiClient
	channelSecret   string
}

func NewHandler(
	channelToken, channelSecret string,
	store *store.FirestoreClient,
	geoClient *geo.GeocodeClient,
	garbageAdapter *garbage.GarbageAdapter,
	geminiClient *gemini.GeminiClient,
) (*Handler, error) {
	messagingAPI, err := messaging_api.NewMessagingApiAPI(channelToken)
	if err != nil {
		return nil, err
	}

	return &Handler{
		messagingAPI:   messagingAPI,
		store:          store,
		geoClient:      geoClient,
		garbageAdapter: garbageAdapter,
		geminiClient:   geminiClient,
		channelSecret:  channelSecret,
	}, nil
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received webhook request from %s", r.RemoteAddr)
	
	cb, err := webhook.ParseRequest(h.channelSecret, r)
	if err != nil {
		log.Printf("Cannot parse request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("Successfully parsed webhook, processing %d events", len(cb.Events))

	for i, event := range cb.Events {
		log.Printf("Processing event %d/%d, type: %T", i+1, len(cb.Events), event)
		
		switch e := event.(type) {
		case webhook.MessageEvent:
			log.Printf("Handling MessageEvent")
			h.handleMessageEvent(r.Context(), e)
		case webhook.PostbackEvent:
			log.Printf("Handling PostbackEvent")
			h.handlePostbackEvent(r.Context(), e)
		default:
			log.Printf("Unhandled event type: %T", event)
		}
	}

	log.Printf("Webhook processing completed successfully")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) getUserID(source webhook.SourceInterface) string {
	switch s := source.(type) {
	case webhook.UserSource:
		log.Printf("User source detected, User ID: %s", s.UserId)
		return s.UserId
	case webhook.GroupSource:
		log.Printf("Group source detected, Group ID: %s", s.GroupId)
		// For group messages, we could potentially handle them differently
		// For now, we ignore group messages
		return ""
	case webhook.RoomSource:
		log.Printf("Room source detected, Room ID: %s", s.RoomId)
		// For room messages, we could potentially handle them differently
		// For now, we ignore room messages
		return ""
	default:
		log.Printf("Unknown source type: %T", source)
		return ""
	}
}

func (h *Handler) handleMessageEvent(ctx context.Context, event webhook.MessageEvent) {
	log.Printf("Processing MessageEvent")
	log.Printf("Message type: %T", event.Message)
	log.Printf("Source type: %T", event.Source)
	
	// First check if we can handle this message type
	switch message := event.Message.(type) {
	case webhook.TextMessageContent:
		log.Printf("Text message received: %s", message.Text)
		// Now get the user ID for text messages
		userID := h.getUserID(event.Source)
		if userID == "" {
			log.Printf("Cannot get user ID from source type %T, ignoring text message", event.Source)
			return
		}
		h.handleTextMessage(ctx, userID, message.Text)
		
	case webhook.LocationMessageContent:
		log.Printf("Location message received: lat=%f, lng=%f, address=%s", message.Latitude, message.Longitude, message.Address)
		// Now get the user ID for location messages
		userID := h.getUserID(event.Source)
		if userID == "" {
			log.Printf("Cannot get user ID from source type %T, ignoring location message", event.Source)
			return
		}
		h.handleLocationMessage(ctx, userID, message.Latitude, message.Longitude, message.Address)
		
	default:
		log.Printf("Unhandled message type: %T", event.Message)
	}
}

func (h *Handler) handleTextMessage(ctx context.Context, userID, text string) {
	log.Printf("Processing text message from user %s: %s", userID, text)
	
	if strings.HasPrefix(text, "/") {
		log.Printf("Command detected: %s", text)
		h.handleCommand(ctx, userID, text)
		return
	}

	// Handle common greetings
	lowerText := strings.ToLower(strings.TrimSpace(text))
	if lowerText == "hi" || lowerText == "hello" || lowerText == "你好" || lowerText == "哈囉" {
		log.Printf("Greeting detected: %s", text)
		welcomeMsg := `👋 您好！歡迎使用垃圾車助手！

🚀 快速開始：
📍 點擊下方「+」按鈕 → 選擇「位置」→「即時位置」
💬 或直接輸入地址，例如：「台北市信義區」

我會幫您找到最近的垃圾車站點和時間！

輸入 /help 查看更多功能`
		h.replyMessage(ctx, userID, welcomeMsg)
		return
	}

	log.Printf("Analyzing intent for text: %s", text)
	intent, err := h.geminiClient.AnalyzeIntent(ctx, text)
	if err != nil {
		log.Printf("Error analyzing intent for user %s: %v", userID, err)
		// 意圖分析失敗時，仍然嘗試作為地址處理
		intent = nil
	}
	
	log.Printf("Intent analysis result: %+v", intent)

	// 首先檢查是否是收藏地點名稱
	favorite := h.findUserFavoriteByName(ctx, userID, text)
	if favorite != nil {
		log.Printf("Found favorite location '%s' for user %s: lat=%f, lng=%f", text, userID, favorite.Lat, favorite.Lng)
		h.searchNearbyGarbageTrucks(ctx, userID, favorite.Lat, favorite.Lng, intent)
		return
	}
	
	// 檢查是否有時間窗口查詢但沒有地址  
	if intent != nil && (intent.TimeWindow.From != "" || intent.TimeWindow.To != "") && intent.District == "" {
		log.Printf("Time window query detected without specific location: %s", text)
		h.handleTimeQueryWithoutLocation(ctx, userID, intent)
		return
	}

	// 嘗試多種方式提取地址
	var addressToGeocode string
	var addressMethod string

	// 方法1：優先使用原始文字作為地址（最準確）
	addressToGeocode = text
	addressMethod = "original.text"
	log.Printf("Method 1 - Using original text as address: %s", addressToGeocode)
	
	// 進行地理編碼
	log.Printf("Geocoding address: '%s' using method: %s", addressToGeocode, addressMethod)
	location, err := h.geoClient.GeocodeAddress(ctx, addressToGeocode)
	if err != nil {
		log.Printf("Error geocoding address '%s' (method: %s) for user %s: %v", addressToGeocode, addressMethod, userID, err)

		// Fallback 1: 嘗試使用 Gemini ExtractLocationFromText
		extractedLocation, extractErr := h.geminiClient.ExtractLocationFromText(ctx, text)
		if extractErr == nil && extractedLocation != "" && strings.TrimSpace(extractedLocation) != text {
			log.Printf("Fallback 1: trying extracted location: %s", extractedLocation)
			location, err = h.geoClient.GeocodeAddress(ctx, extractedLocation)
			if err == nil {
				log.Printf("Fallback 1 geocoding succeeded with extracted location")
				h.searchNearbyGarbageTrucks(ctx, userID, location.Lat, location.Lng, intent)
				return
			}
			log.Printf("Fallback 1 geocoding failed: %v", err)
		}

		// Fallback 2: 嘗試簡化地址（提取縣市區）
		simplifiedAddress := h.extractSimplifiedAddress(text)
		if simplifiedAddress != "" && simplifiedAddress != text {
			log.Printf("Fallback 2: trying simplified address: %s", simplifiedAddress)
			location, err = h.geoClient.GeocodeAddress(ctx, simplifiedAddress)
			if err == nil {
				log.Printf("Fallback 2 geocoding succeeded with simplified address")
				h.searchNearbyGarbageTrucks(ctx, userID, location.Lat, location.Lng, intent)
				return
			}
			log.Printf("Fallback 2 geocoding failed: %v", err)
		}

		// Fallback 3: 如果有 intent.District，嘗試使用它
		if intent != nil && intent.District != "" && intent.District != text {
			log.Printf("Fallback 3: trying intent district: %s", intent.District)
			location, err = h.geoClient.GeocodeAddress(ctx, intent.District)
			if err == nil {
				log.Printf("Fallback 3 geocoding succeeded with intent district")
				h.searchNearbyGarbageTrucks(ctx, userID, location.Lat, location.Lng, intent)
				return
			}
			log.Printf("Fallback 3 geocoding failed: %v", err)
		}

		h.replyMessage(ctx, userID, fmt.Sprintf("抱歉，我找不到「%s」的位置資訊。\n\n💡 請嘗試：\n📍 分享您的位置\n💬 輸入更具體的地址（如：台北市信義區忠孝東路）\n🔍 或者搜尋：「台北市中正區」", text))
		return
	}
	
	log.Printf("Geocoded successfully: %+v", location)
	h.searchNearbyGarbageTrucks(ctx, userID, location.Lat, location.Lng, intent)
}

func (h *Handler) handleTimeQueryWithoutLocation(ctx context.Context, userID string, intent *gemini.IntentResult) {
	fromTime, toTime, err := h.geminiClient.ParseTimeWindow(intent.TimeWindow)
	if err != nil {
		log.Printf("Error parsing time window: %v", err)
		h.replyMessage(ctx, userID, "抱歉，無法理解您指定的時間。")
		return
	}

	var timeDesc string
	if !toTime.IsZero() {
		timeDesc = fmt.Sprintf("%s前", toTime.Format("15:04"))
	} else if !fromTime.IsZero() {
		timeDesc = fmt.Sprintf("%s後", fromTime.Format("15:04"))
	} else {
		timeDesc = "指定時間內"
	}

	// 檢查用戶是否有收藏地點
	user, err := h.store.GetUser(ctx, userID)
	if err == nil && len(user.Favorites) > 0 {
		// 用戶有收藏地點，提供選項
		message := fmt.Sprintf("🕐 您想查詢%s的垃圾車資訊\n\n您可以：\n", timeDesc)
		message += "📍 分享您的即時位置\n"
		message += "❤️ 選擇收藏地點：\n"
		
		for i, fav := range user.Favorites {
			if i >= 3 { // 限制顯示前3個收藏
				break
			}
			message += fmt.Sprintf("• %s\n", fav.Name)
		}
		message += "\n請分享位置或輸入收藏地點名稱"
		h.replyMessage(ctx, userID, message)
	} else {
		// 用戶沒有收藏地點
		message := fmt.Sprintf("🕐 您想查詢%s的垃圾車資訊\n\n", timeDesc)
		message += "請提供位置資訊：\n"
		message += "📍 分享您的即時位置，或\n"
		message += "💬 輸入具體地址\n\n"
		message += "💡 您也可以使用 `/favorite 家 台北市大安區xxx` 來收藏常用地點"
		h.replyMessage(ctx, userID, message)
	}
}

func (h *Handler) handleLocationMessage(ctx context.Context, userID string, lat, lng float64, address string) {
	log.Printf("Received location from user %s: lat=%f, lng=%f, address=%s", userID, lat, lng, address)
	
	// If no address provided by LINE, try reverse geocoding
	if address == "" {
		location, err := h.geoClient.ReverseGeocode(ctx, lat, lng)
		if err != nil {
			log.Printf("Error reverse geocoding location: %v", err)
			// Continue with empty address - we still have coordinates
		} else {
			address = location.Address
			log.Printf("Reverse geocoded address: %s", address)
		}
	}
	
	// Send a friendly confirmation message with the address
	var confirmMsg string
	if address != "" {
		confirmMsg = fmt.Sprintf("📍 收到您的位置：%s\n\n正在為您查詢附近的垃圾車...", address)
	} else {
		confirmMsg = "📍 收到您的位置\n\n正在為您查詢附近的垃圾車..."
	}
	h.replyMessage(ctx, userID, confirmMsg)
	
	// Search for nearby garbage trucks and offer to save location
	h.searchNearbyGarbageTrucksWithSaveOption(ctx, userID, lat, lng, address, nil)
}

func (h *Handler) searchNearbyGarbageTrucksWithSaveOption(ctx context.Context, userID string, lat, lng float64, address string, intent *gemini.IntentResult) {
	// 先搜尋垃圾車
	h.searchNearbyGarbageTrucks(ctx, userID, lat, lng, intent)
	
	// 然後詢問是否要收藏此位置
	if address != "" {
		h.offerLocationSave(ctx, userID, lat, lng, address)
	}
}

func (h *Handler) offerLocationSave(ctx context.Context, userID string, lat, lng float64, address string) {
	// 檢查是否已經收藏過相近的位置
	user, err := h.store.GetUser(ctx, userID)
	if err == nil {
		for _, fav := range user.Favorites {
			// 檢查相近位置（100公尺內）
			distance := geo.CalculateDistance(lat, lng, fav.Lat, fav.Lng)
			if distance < 100 {
				// 已有相近位置，不再詢問
				return
			}
		}
	}

	// 建議收藏地點名稱
	suggestedName := h.suggestLocationName(address)
	
	favoriteData := fmt.Sprintf("action=add_favorite&lat=%f&lng=%f&name=%s&address=%s", 
		lat, lng, suggestedName, address)

	// 創建詢問收藏的 Flex Message
	bubble := messaging_api.FlexBubble{
		Body: &messaging_api.FlexBox{
			Layout: "vertical",
			Contents: []messaging_api.FlexComponentInterface{
				&messaging_api.FlexText{
					Text:   "💡 要收藏這個位置嗎？",
					Weight: "bold",
					Size:   "md",
				},
				&messaging_api.FlexText{
					Text:  fmt.Sprintf("📍 %s", address),
					Size:  "sm",
					Color: "#666666",
					Wrap:  true,
				},
				&messaging_api.FlexText{
					Text:  "收藏後可以直接輸入地點名稱快速查詢！",
					Size:  "xs",
					Color: "#999999",
					Wrap:  true,
				},
			},
		},
		Footer: &messaging_api.FlexBox{
			Layout: "horizontal",
			Contents: []messaging_api.FlexComponentInterface{
				&messaging_api.FlexButton{
					Action: &messaging_api.PostbackAction{
						Label: "⭐ 收藏",
						Data:  favoriteData,
					},
					Style: "primary",
					Flex:  2,
				},
				&messaging_api.FlexButton{
					Action: &messaging_api.PostbackAction{
						Label: "暫時不要",
						Data:  "action=dismiss_save",
					},
					Style: "secondary",
					Flex:  1,
				},
			},
		},
	}

	flexMessage := messaging_api.FlexMessage{
		AltText:  "收藏位置建議",
		Contents: &bubble,
	}

	h.sendMessage(ctx, userID, &flexMessage)
}

func (h *Handler) suggestLocationName(address string) string {
	// 簡單的地點名稱建議邏輯
	if strings.Contains(address, "家") || strings.Contains(address, "住") {
		return "家"
	}
	if strings.Contains(address, "公司") || strings.Contains(address, "辦公") {
		return "公司"
	}
	if strings.Contains(address, "學校") || strings.Contains(address, "大學") {
		return "學校"
	}
	
	// 提取區域名稱作為建議
	parts := strings.Split(address, " ")
	if len(parts) > 0 {
		firstPart := parts[0]
		if len(firstPart) > 10 {
			// 如果太長，取前面部分
			return firstPart[:10] + "..."
		}
		return firstPart
	}
	
	return "新地點"
}

func (h *Handler) handleCommand(ctx context.Context, userID, command string) {
	parts := strings.Split(command, " ")
	cmd := parts[0]

	switch cmd {
	case "/help":
		helpText := `歡迎使用垃圾車助手！

🚛 查詢垃圾車：
📍 分享位置：點擊「+」→「位置」→「即時位置」
💬 輸入地址：「台北市大安區忠孝東路」
🕐 時間查詢：「我晚上七點前在哪裡倒垃圾？」

⭐ 收藏管理：
/list - 查看收藏清單（含互動按鈕）
/favorite 家 台北市大安區 - 新增收藏
/delete 家 - 刪除收藏

⏰ 提醒功能：
點擊查詢結果中的「提醒我」按鈕設定通知

💡 更快速的收藏方式：
🔸 分享位置後點擊「⭐ 收藏」
🔸 查詢結果中點擊「收藏此地點」`
		h.replyMessage(ctx, userID, helpText)

	case "/favorite", "/add", "/save":
		if len(parts) < 2 {
			h.replyMessage(ctx, userID, "請使用：/favorite [地點名稱] [地址]\n\n💡 或者您可以：\n📍 分享位置後點擊收藏\n🗑️ 查詢垃圾車後點擊「收藏此地點」")
			return
		}
		name := parts[1]
		address := strings.Join(parts[2:], " ")
		h.addFavorite(ctx, userID, name, address)

	case "/list":
		h.listFavoritesWithUI(ctx, userID)
		
	case "/delete", "/remove":
		if len(parts) < 2 {
			h.replyMessage(ctx, userID, "請使用：/delete [地點名稱]")
			return
		}
		name := strings.Join(parts[1:], " ")
		h.deleteFavorite(ctx, userID, name)

	default:
		h.replyMessage(ctx, userID, "未知指令。請使用 /help 查看可用指令。")
	}
}

func (h *Handler) searchNearbyGarbageTrucks(ctx context.Context, userID string, lat, lng float64, intent *gemini.IntentResult) {
	log.Printf("Searching nearby garbage trucks for user %s at coordinates: lat=%f, lng=%f", userID, lat, lng)
	
	garbageData, err := h.garbageAdapter.FetchGarbageData(ctx)
	if err != nil {
		log.Printf("Error fetching garbage data for user %s: %v", userID, err)
		h.replyMessage(ctx, userID, "抱歉，無法取得垃圾車資料。")
		return
	}
	
	log.Printf("Successfully fetched garbage data, %d collection points available", len(garbageData.Result.Results))

	var nearestStops []*garbage.NearestStop

	if intent != nil && (intent.TimeWindow.From != "" || intent.TimeWindow.To != "") {
		log.Printf("Time window query detected: from=%s, to=%s", intent.TimeWindow.From, intent.TimeWindow.To)
		fromTime, toTime, err := h.geminiClient.ParseTimeWindow(intent.TimeWindow)
		if err == nil {
			log.Printf("Parsed time window: from=%v, to=%v", fromTime, toTime)
			timeWindow := garbage.TimeWindow{From: fromTime, To: toTime}
			nearestStops, err = h.garbageAdapter.FindStopsInTimeWindow(lat, lng, garbageData, timeWindow, 2000)
			log.Printf("Found %d stops in time window", len(nearestStops))
		} else {
			log.Printf("Error parsing time window: %v", err)
		}
	}

	if len(nearestStops) == 0 {
		log.Printf("No stops found in time window, searching for nearest stops")
		nearestStops, err = h.garbageAdapter.FindNearestStops(lat, lng, garbageData, 5)
		if err != nil {
			log.Printf("Error finding nearest stops for user %s: %v", userID, err)
			h.replyMessage(ctx, userID, "抱歉，無法找到附近的垃圾車站點。")
			return
		}
		log.Printf("Found %d nearest stops", len(nearestStops))
	}

	if len(nearestStops) == 0 {
		log.Printf("No garbage truck stops found for user %s at coordinates lat=%f, lng=%f", userID, lat, lng)
		h.replyMessage(ctx, userID, "附近沒有找到垃圾車站點。")
		return
	}

	log.Printf("Sending %d garbage truck results to user %s", len(nearestStops), userID)
	h.sendGarbageTruckResults(ctx, userID, nearestStops)
}

func (h *Handler) sendGarbageTruckResults(ctx context.Context, userID string, stops []*garbage.NearestStop) {
	log.Printf("Preparing to send garbage truck results to user %s", userID)
	
	if len(stops) == 0 {
		log.Printf("No stops to send to user %s", userID)
		return
	}

	var bubbles []messaging_api.FlexBubble

	for i, stop := range stops {
		if i >= 3 {
			log.Printf("Limiting results to first 3 stops")
			break
		}

		log.Printf("Creating bubble for stop %d: %s", i+1, stop.Stop.Name)
		bubble := h.createGarbageTruckBubble(stop)
		bubbles = append(bubbles, bubble)
	}

	log.Printf("Created %d bubbles for user %s", len(bubbles), userID)
	
	carousel := messaging_api.FlexCarousel{
		Contents: bubbles,
	}

	flexMessage := messaging_api.FlexMessage{
		AltText:  "垃圾車查詢結果",
		Contents: &carousel,
	}

	log.Printf("Sending flex message with %d bubbles to user %s", len(bubbles), userID)
	h.sendMessage(ctx, userID, &flexMessage)
}

func (h *Handler) createGarbageTruckBubble(stop *garbage.NearestStop) messaging_api.FlexBubble {
	timeStr := stop.ETA.Format("15:04")
	distanceStr := geo.FormatDistance(stop.Distance)
	directionsURL := h.geoClient.GetDirectionsURL(stop.Stop.Lat, stop.Stop.Lng)

	reminderData := fmt.Sprintf("route=%s&stop=%s&eta=%d", 
		stop.Route.ID, stop.Stop.Name, stop.ETA.Unix())

	body := messaging_api.FlexBox{
		Layout: "vertical",
		Contents: []messaging_api.FlexComponentInterface{
			&messaging_api.FlexText{
				Text:   stop.Stop.Name,
				Weight: "bold",
				Size:   "lg",
			},
			&messaging_api.FlexText{
				Text: fmt.Sprintf("下一班：%s", timeStr),
				Size: "md",
			},
			&messaging_api.FlexText{
				Text:  fmt.Sprintf("距離：%s", distanceStr),
				Size:  "sm",
				Color: "#888888",
			},
			&messaging_api.FlexText{
				Text:  fmt.Sprintf("路線：%s", stop.Route.Name),
				Size:  "sm",
				Color: "#888888",
			},
		},
	}

	favoriteData := fmt.Sprintf("action=add_favorite&lat=%f&lng=%f&name=%s&address=%s", 
		stop.Stop.Lat, stop.Stop.Lng, stop.Stop.Name, stop.Stop.Name)

	footer := messaging_api.FlexBox{
		Layout: "vertical",
		Contents: []messaging_api.FlexComponentInterface{
			&messaging_api.FlexBox{
				Layout: "horizontal",
				Contents: []messaging_api.FlexComponentInterface{
					&messaging_api.FlexButton{
						Action: &messaging_api.UriAction{
							Label: "導航",
							Uri:   directionsURL,
						},
						Style: "secondary",
						Flex:  2,
					},
					&messaging_api.FlexButton{
						Action: &messaging_api.PostbackAction{
							Label: "提醒我",
							Data:  reminderData,
						},
						Style: "primary",
						Flex:  2,
					},
				},
			},
			&messaging_api.FlexButton{
				Action: &messaging_api.PostbackAction{
					Label: "⭐ 收藏此地點",
					Data:  favoriteData,
				},
				Style: "link",
				Color: "#999999",
			},
		},
	}

	return messaging_api.FlexBubble{
		Body:   &body,
		Footer: &footer,
	}
}

func (h *Handler) handlePostbackEvent(ctx context.Context, event webhook.PostbackEvent) {
	log.Printf("Processing PostbackEvent")
	log.Printf("Source type: %T", event.Source)
	log.Printf("Postback data: %s", event.Postback.Data)
	
	userID := h.getUserID(event.Source)
	if userID == "" {
		log.Printf("Cannot get user ID from source type %T, ignoring postback event", event.Source)
		return
	}

	data := event.Postback.Data
	params := parsePostbackData(data)

	// 處理收藏功能
	if action, ok := params["action"]; ok {
		switch action {
		case "add_favorite":
			h.handleAddFavoritePostback(ctx, userID, params)
			return
		case "dismiss_save":
			h.replyMessage(ctx, userID, "好的，如需收藏地點，可使用 `/favorite [名稱] [地址]` 指令")
			return
		case "query_favorite":
			h.handleQueryFavoritePostback(ctx, userID, params)
			return
		case "delete_favorite":
			h.handleDeleteFavoritePostback(ctx, userID, params)
			return
		}
	}

	if routeID, ok := params["route"]; ok {
		stopName := params["stop"]
		etaStr := params["eta"]
		
		eta, err := strconv.ParseInt(etaStr, 10, 64)
		if err != nil {
			h.replyMessage(ctx, userID, "提醒設定失敗：時間格式錯誤")
			return
		}

		etaTime := time.Unix(eta, 0)
		notificationTime := etaTime.Add(-10 * time.Minute)
		
		log.Printf("Creating reminder for user %s: stop=%s, ETA=%s, notificationTime=%s", 
			userID, stopName, etaTime.Format("2006-01-02 15:04:05"), notificationTime.Format("2006-01-02 15:04:05"))
		
		reminder := &store.Reminder{
			UserID:         userID,
			StopName:       stopName,
			RouteID:        routeID,
			ETA:            etaTime,
			AdvanceMinutes: 10,
		}

		err = h.store.CreateReminder(ctx, reminder)
		if err != nil {
			log.Printf("Error creating reminder: %v", err)
			h.replyMessage(ctx, userID, "提醒設定失敗")
			return
		}

		log.Printf("Successfully created reminder for user %s, will notify at %s", userID, notificationTime.Format("2006-01-02 15:04:05"))
		h.replyMessage(ctx, userID, fmt.Sprintf("✅ 已設定提醒！\n將在垃圾車抵達 %s 前 10 分鐘通知您。", stopName))
	}
}

func (h *Handler) handleAddFavoritePostback(ctx context.Context, userID string, params map[string]string) {
	lat, err := strconv.ParseFloat(params["lat"], 64)
	if err != nil {
		h.replyMessage(ctx, userID, "收藏失敗：位置資訊錯誤")
		return
	}

	lng, err := strconv.ParseFloat(params["lng"], 64)
	if err != nil {
		h.replyMessage(ctx, userID, "收藏失敗：位置資訊錯誤")
		return
	}

	stopName := params["name"]
	if stopName == "" {
		h.replyMessage(ctx, userID, "收藏失敗：地點名稱為空")
		return
	}

	// 檢查是否已經收藏過相同地點
	user, err := h.store.GetUser(ctx, userID)
	if err == nil {
		for _, fav := range user.Favorites {
			// 檢查是否已存在相同名稱或相近位置的收藏
			if fav.Name == stopName || (math.Abs(fav.Lat-lat) < 0.001 && math.Abs(fav.Lng-lng) < 0.001) {
				h.replyMessage(ctx, userID, fmt.Sprintf("「%s」已經在您的收藏清單中了！", stopName))
				return
			}
		}
	}

	// 進行反向地理編碼獲取完整地址
	location, err := h.geoClient.ReverseGeocode(ctx, lat, lng)
	var address string
	if err != nil {
		log.Printf("Reverse geocoding failed: %v", err)
		address = fmt.Sprintf("緯度 %f, 經度 %f", lat, lng)
	} else {
		address = location.Address
	}

	favorite := store.Favorite{
		Name:    stopName,
		Lat:     lat,
		Lng:     lng,
		Address: address,
	}

	err = h.store.AddFavorite(ctx, userID, favorite)
	if err != nil {
		log.Printf("Error adding favorite: %v", err)
		h.replyMessage(ctx, userID, "收藏失敗，請稍後再試")
		return
	}

	h.replyMessage(ctx, userID, fmt.Sprintf("⭐ 已收藏「%s」\n📍 %s\n\n💡 您可以直接輸入「%s」來快速查詢此地點的垃圾車資訊", stopName, address, stopName))
}

func (h *Handler) handleQueryFavoritePostback(ctx context.Context, userID string, params map[string]string) {
	lat, err := strconv.ParseFloat(params["lat"], 64)
	if err != nil {
		h.replyMessage(ctx, userID, "查詢失敗：位置資訊錯誤")
		return
	}

	lng, err := strconv.ParseFloat(params["lng"], 64)
	if err != nil {
		h.replyMessage(ctx, userID, "查詢失敗：位置資訊錯誤")
		return
	}

	name := params["name"]
	h.replyMessage(ctx, userID, fmt.Sprintf("🔍 正在為您查詢「%s」附近的垃圾車...", name))
	h.searchNearbyGarbageTrucks(ctx, userID, lat, lng, nil)
}

func (h *Handler) handleDeleteFavoritePostback(ctx context.Context, userID string, params map[string]string) {
	name := params["name"]
	if name == "" {
		h.replyMessage(ctx, userID, "刪除失敗：地點名稱為空")
		return
	}

	h.deleteFavorite(ctx, userID, name)
}

func (h *Handler) findUserFavoriteByName(ctx context.Context, userID, name string) *store.Favorite {
	user, err := h.store.GetUser(ctx, userID)
	if err != nil {
		log.Printf("Error getting user %s: %v", userID, err)
		return nil
	}

	// 進行模糊匹配收藏地點名稱
	lowerName := strings.ToLower(strings.TrimSpace(name))
	for _, fav := range user.Favorites {
		lowerFavName := strings.ToLower(strings.TrimSpace(fav.Name))
		// 完全匹配或包含匹配
		if lowerFavName == lowerName || strings.Contains(lowerFavName, lowerName) || strings.Contains(lowerName, lowerFavName) {
			return &fav
		}
	}
	return nil
}

func (h *Handler) addFavorite(ctx context.Context, userID, name, address string) {
	location, err := h.geoClient.GeocodeAddress(ctx, address)
	if err != nil {
		h.replyMessage(ctx, userID, "無法找到該地址的位置資訊")
		return
	}

	favorite := store.Favorite{
		Name:    name,
		Lat:     location.Lat,
		Lng:     location.Lng,
		Address: location.Address,
	}

	err = h.store.AddFavorite(ctx, userID, favorite)
	if err != nil {
		log.Printf("Error adding favorite: %v", err)
		h.replyMessage(ctx, userID, "收藏地點失敗")
		return
	}

	h.replyMessage(ctx, userID, fmt.Sprintf("✅ 已收藏地點：%s", name))
}

func (h *Handler) listFavorites(ctx context.Context, userID string) {
	user, err := h.store.GetUser(ctx, userID)
	if err != nil {
		h.replyMessage(ctx, userID, "無法取得收藏清單")
		return
	}

	if len(user.Favorites) == 0 {
		h.replyMessage(ctx, userID, "您還沒有收藏任何地點")
		return
	}

	var message strings.Builder
	message.WriteString("您的收藏地點：\n\n")
	for i, fav := range user.Favorites {
		message.WriteString(fmt.Sprintf("%d. %s\n   %s\n\n", i+1, fav.Name, fav.Address))
	}

	h.replyMessage(ctx, userID, message.String())
}

func (h *Handler) listFavoritesWithUI(ctx context.Context, userID string) {
	user, err := h.store.GetUser(ctx, userID)
	if err != nil {
		h.replyMessage(ctx, userID, "無法取得收藏清單")
		return
	}

	if len(user.Favorites) == 0 {
		welcomeMsg := `您還沒有收藏任何地點

💡 如何新增收藏：
📍 分享位置後點擊「⭐ 收藏」
🗑️ 查詢垃圾車後點擊「收藏此地點」  
💬 使用指令：/favorite 家 台北市大安區`
		h.replyMessage(ctx, userID, welcomeMsg)
		return
	}

	// 創建收藏清單的 Flex Message
	var bubbles []messaging_api.FlexBubble
	
	for i, fav := range user.Favorites {
		if i >= 10 { // 限制最多顯示10個收藏
			break
		}
		
		bubble := h.createFavoriteBubble(fav)
		bubbles = append(bubbles, bubble)
	}

	carousel := messaging_api.FlexCarousel{
		Contents: bubbles,
	}

	flexMessage := messaging_api.FlexMessage{
		AltText:  fmt.Sprintf("您的收藏清單 (%d個地點)", len(user.Favorites)),
		Contents: &carousel,
	}

	h.sendMessage(ctx, userID, &flexMessage)
}

func (h *Handler) createFavoriteBubble(fav store.Favorite) messaging_api.FlexBubble {
	// 截短地址顯示
	shortAddress := fav.Address
	if len(shortAddress) > 30 {
		shortAddress = shortAddress[:30] + "..."
	}

	queryData := fmt.Sprintf("action=query_favorite&lat=%f&lng=%f&name=%s", 
		fav.Lat, fav.Lng, fav.Name)
	deleteData := fmt.Sprintf("action=delete_favorite&name=%s", fav.Name)

	body := messaging_api.FlexBox{
		Layout: "vertical",
		Contents: []messaging_api.FlexComponentInterface{
			&messaging_api.FlexText{
				Text:   fav.Name,
				Weight: "bold",
				Size:   "lg",
				Color:  "#333333",
			},
			&messaging_api.FlexText{
				Text:  shortAddress,
				Size:  "sm",
				Color: "#666666",
				Wrap:  true,
			},
		},
	}

	footer := messaging_api.FlexBox{
		Layout: "vertical",
		Contents: []messaging_api.FlexComponentInterface{
			&messaging_api.FlexBox{
				Layout: "horizontal",
				Contents: []messaging_api.FlexComponentInterface{
					&messaging_api.FlexButton{
						Action: &messaging_api.PostbackAction{
							Label: "🚛 查詢垃圾車",
							Data:  queryData,
						},
						Style: "primary",
						Flex:  3,
					},
					&messaging_api.FlexButton{
						Action: &messaging_api.PostbackAction{
							Label: "🗑️ 刪除",
							Data:  deleteData,
						},
						Style: "secondary",
						Flex:  1,
					},
				},
			},
		},
	}

	return messaging_api.FlexBubble{
		Body:   &body,
		Footer: &footer,
	}
}

func (h *Handler) deleteFavorite(ctx context.Context, userID, name string) {
	user, err := h.store.GetUser(ctx, userID)
	if err != nil {
		h.replyMessage(ctx, userID, "無法取得收藏清單")
		return
	}

	// 查找要刪除的收藏
	found := false
	var newFavorites []store.Favorite
	for _, fav := range user.Favorites {
		if strings.EqualFold(strings.TrimSpace(fav.Name), strings.TrimSpace(name)) {
			found = true
			continue // 跳過這個收藏（等於刪除）
		}
		newFavorites = append(newFavorites, fav)
	}

	if !found {
		h.replyMessage(ctx, userID, fmt.Sprintf("找不到名為「%s」的收藏地點", name))
		return
	}

	// 更新用戶收藏清單
	user.Favorites = newFavorites
	err = h.store.UpsertUser(ctx, user)
	if err != nil {
		log.Printf("Error updating user favorites: %v", err)
		h.replyMessage(ctx, userID, "刪除收藏失敗，請稍後再試")
		return
	}

	h.replyMessage(ctx, userID, fmt.Sprintf("✅ 已刪除收藏「%s」", name))
}

func (h *Handler) extractSimplifiedAddress(text string) string {
	// 嘗試提取縣市區的模式
	patterns := []string{
		`(台北市|新北市|桃園市|台中市|台南市|高雄市|基隆市|新竹市|嘉義市)[^市]*?(區|市)`,
		`(新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|台東縣|澎湖縣|金門縣|連江縣)[^縣]*?(鄉|鎮|市)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(text); match != "" {
			log.Printf("Extracted simplified address using pattern: %s -> %s", pattern, match)
			return match
		}
	}
	
	// 如果沒有匹配，嘗試提取縣市
	cityPattern := `(台北市|新北市|桃園市|台中市|台南市|高雄市|基隆市|新竹市|嘉義市|新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|台東縣|澎湖縣|金門縣|連江縣)`
	re := regexp.MustCompile(cityPattern)
	if match := re.FindString(text); match != "" {
		log.Printf("Extracted city from address: %s", match)
		return match
	}
	
	return ""
}

func (h *Handler) replyMessage(ctx context.Context, userID, text string) {
	log.Printf("Sending reply to user %s: %s", userID, text)
	message := messaging_api.TextMessage{
		Text: text,
	}
	h.sendMessage(ctx, userID, &message)
}

func (h *Handler) sendMessage(ctx context.Context, userID string, message messaging_api.MessageInterface) {
	log.Printf("Attempting to send message to user: %s", userID)
	
	req := &messaging_api.PushMessageRequest{
		To:       userID,
		Messages: []messaging_api.MessageInterface{message},
	}

	log.Printf("Calling LINE Messaging API...")
	resp, err := h.messagingAPI.PushMessage(req, "")
	if err != nil {
		log.Printf("Error sending message to user %s: %v", userID, err)
		return
	}
	
	log.Printf("Message sent successfully to user %s. Response: %+v", userID, resp)
}

func (h *Handler) GetMessagingAPI() *messaging_api.MessagingApiAPI {
	return h.messagingAPI
}

func parsePostbackData(data string) map[string]string {
	params := make(map[string]string)
	pairs := strings.Split(data, "&")
	
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	
	return params
}
