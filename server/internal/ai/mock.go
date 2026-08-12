package ai

import (
	"context"
	"strings"
	"time"
)

// MockLLM 无 API Key 时的兜底实现：返回预置的高质量多角色场景内容
// 保证系统无外部依赖也可端到端演示；配置 LLM_API_KEY 后自动切换真实模型
type MockLLM struct{}

func (m *MockLLM) ChatJSON(_ context.Context, system, user string) (string, error) {
	// 简单按关键词匹配场景，默认餐厅点餐
	switch {
	case containsAny(user, "机场", "airport", "Airport", "旅行", "flight"):
		return mockAirportContent, nil
	case containsAny(user, "购物", "shopping", "Shop", "store"):
		return mockShoppingContent, nil
	default:
		return mockRestaurantContent, nil
	}
}

// Test mock 模式连接测试
func (m *MockLLM) Test(_ context.Context) (string, error) {
	return "mock 模式（未配置 LLM_API_KEY，使用内置示例内容）", nil
}

// mockReplies 无 Key 时的模拟对话回复（按轮次轮换 + 关键词回应）
var mockReplies = []string{
	"That sounds great! Could you tell me a bit more?",
	"Oh, I see! What would you like to do next?",
	"Good idea! And would you like anything else?",
	"Sure! Let me think about that for a moment.",
	"Nice! How does that sound to you?",
	"Of course. Is there anything else I can help with?",
}

// mockReply 生成模拟对话回复（关键词回应 + 轮换模板）
func mockReply(user string) string {
	u := strings.ToLower(user)
	switch {
	case containsAny(u, "menu", "order", "food", "eat"):
		return "Of course! Today's special is grilled salmon with lemon. Would you like to try it?"
	case containsAny(u, "price", "much", "cost", "dollar"):
		return "It's very reasonable — only five dollars and fifty cents. Great value!"
	case containsAny(u, "thank", "thanks"):
		return "You're very welcome! Enjoy your meal!"
	case containsAny(u, "help", "what", "where", "how", "can you"):
		return "Sure! You can ask me anything about the menu, the food, or the restaurant."
	case containsAny(u, "hello", "hi", "good", "morning"):
		return "Hello! Great to see you. How can I help you today?"
	case containsAny(u, "bye", "goodbye", "see you"):
		return "Goodbye! It was lovely talking to you. Have a wonderful day!"
	default:
		return mockReplies[len(u)%len(mockReplies)]
	}
}

// Chat mock 对话：关键词回应 + 轮换模板，保证无 Key 也能体验完整对话
func (m *MockLLM) Chat(_ context.Context, system, user string) (string, error) {
	return mockReply(user), nil
}

// ChatStream mock 流式：按 2-4 字符分片模拟打字机效果
func (m *MockLLM) ChatStream(_ context.Context, system, user string, onDelta StreamDelta) error {
	reply := mockReply(user)
	runes := []rune(reply)
	for i := 0; i < len(runes); {
		n := 1 + i%3 // 2-4 字符分片，制造节奏感
		if i+n > len(runes) {
			n = len(runes) - i
		}
		if err := onDelta(string(runes[i : i+n])); err != nil {
			return err
		}
		i += n
		time.Sleep(15 * time.Millisecond)
	}
	return nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

const mockRestaurantContent = `{
  "unit": "Unit 3",
  "title": "At the Restaurant",
  "scene": "餐厅点餐",
  "dialogue": [
    {"role": "waiter", "role_name": "服务员", "text": "Good evening! Welcome to Green Garden Restaurant. Can I take your order?", "image": ""},
    {"role": "customer", "role_name": "顾客", "text": "Good evening! Could I see the menu, please?", "image": ""},
    {"role": "waiter", "role_name": "服务员", "text": "Of course. Here you are. Today's special is grilled salmon with lemon.", "image": ""},
    {"role": "customer", "role_name": "顾客", "text": "That sounds great. I'd like the grilled salmon, please. And a glass of orange juice.", "image": ""},
    {"role": "waiter", "role_name": "服务员", "text": "Excellent choice! Would you like anything else, like a dessert?", "image": ""},
    {"role": "customer", "role_name": "顾客", "text": "Yes, I'd love a piece of chocolate cake, please.", "image": ""},
    {"role": "waiter", "role_name": "服务员", "text": "Sure. So that's grilled salmon, orange juice and chocolate cake. I'll be right back!", "image": ""}
  ],
  "key_vocab": [
    {"word": "menu", "phonetic": "/ˈmenjuː/", "meaning": "菜单", "example": "Could I see the menu, please?"},
    {"word": "order", "phonetic": "/ˈɔːrdər/", "meaning": "点餐；订单", "example": "Can I take your order?"},
    {"word": "special", "phonetic": "/ˈspeʃl/", "meaning": "特价菜；特别的", "example": "Today's special is grilled salmon."},
    {"word": "dessert", "phonetic": "/dɪˈzɜːrt/", "meaning": "甜点", "example": "Would you like a dessert?"},
    {"word": "juice", "phonetic": "/dʒuːs/", "meaning": "果汁", "example": "A glass of orange juice, please."}
  ],
  "grammar_points": [
    {"point": "Would like（想要）", "explanation": "表达礼貌地想要某物，比 want 更委婉。后面接名词或 to do。", "example": "I'd like the grilled salmon. / I'd like to order now."},
    {"point": "Can I...?（请求许可）", "explanation": "礼貌请求做某事，日常高频表达。", "example": "Can I take your order? / Can I have the menu?"}
  ],
  "practice": [
    {"type": "roleplay", "prompt": "你是顾客，用学过的句型向服务员点一份牛排和一杯咖啡。", "example": "I'd like a steak and a cup of coffee, please.", "hints": ["I'd like...", "Could I have...?"]},
    {"type": "speak", "prompt": "大声朗读这段对话，注意 Would like 和 Could I 的连读。", "example": "朗读 dialogue 全篇", "hints": ["I'd like = I would like", "弱读：could I → /kəd aɪ/"]},
    {"type": "write", "prompt": "用 3 句话写一段你自己在餐厅点餐的小对话。", "example": "Waiter: May I help you? You: Yes, I'd like a salad, please.", "hints": ["用到 would like / could I / please"]}
  ],
  "images": [
    "https://images.unsplash.com/photo-1517248135467-4c7edcad34c4?w=800",
    "https://images.unsplash.com/photo-1414235077428-338989a2e8c0?w=800"
  ],
  "image_prompts": [
    "a cozy family restaurant interior with a friendly waiter taking an order from a customer at a table with menus, warm golden lighting, realistic illustration style",
    "a close-up of a delicious grilled salmon with lemon and a glass of orange juice on a restaurant table, chocolate cake dessert, appetizing food photography style"
  ],
  "svg_images": [
    {
      "title": "点餐对话流程",
      "svg": "<svg viewBox='0 0 400 260' xmlns='http://www.w3.org/2000/svg'><rect width='400' height='260' fill='#fdf6ec' rx='12'/><text x='200' y='30' text-anchor='middle' font-size='16' font-weight='bold' fill='#8a5a00'>Ordering Food Flow</text><rect x='30' y='60' width='150' height='40' rx='8' fill='#1a73e8'/><text x='105' y='86' text-anchor='middle' font-size='14' fill='#fff'>Waiter</text><path d='M 180 80 L 250 80' stroke='#888' stroke-width='2'/><rect x='250' y='60' width='130' height='40' rx='8' fill='#188038'/><text x='315' y='86' text-anchor='middle' font-size='14' fill='#fff'>Menu?</text><path d='M 220 130 L 315 130' stroke='#888' stroke-width='2'/><rect x='30' y='110' width='190' height='40' rx='8' fill='#e8f0fe' stroke='#1a73e8'/><text x='125' y='136' text-anchor='middle' font-size='14' fill='#1a73e8'>I'd like...</text><path d='M 220 190 L 315 190' stroke='#888' stroke-width='2'/><rect x='250' y='170' width='130' height='40' rx='8' fill='#f9ab00'/><text x='315' y='196' text-anchor='middle' font-size='14' fill='#fff'>Dessert?</text><rect x='30' y='210' width='190' height='40' rx='8' fill='#fff3e0' stroke='#b06000'/><text x='125' y='236' text-anchor='middle' font-size='14' fill='#8a5a00'>Chocolate cake</text><text x='200' y='256' text-anchor='middle' font-size='11' fill='#999'>Waiter → Customer → Waiter → Customer</text></svg>"
    }
  ]
}`

const mockShoppingContent = `{
  "unit": "Unit 5",
  "title": "At the Supermarket",
  "scene": "超市购物",
  "dialogue": [
    {"role": "clerk", "role_name": "店员", "text": "Hi there! Are you finding everything okay?", "image": ""},
    {"role": "customer", "role_name": "顾客", "text": "Yes, thank you. Excuse me, how much is this milk?", "image": ""},
    {"role": "clerk", "role_name": "店员", "text": "It's two dollars and fifty cents. It's on sale this week.", "image": ""},
    {"role": "customer", "role_name": "顾客", "text": "Great, I'll take two bottles. Where can I find the bread?", "image": ""},
    {"role": "clerk", "role_name": "店员", "text": "It's in aisle three, next to the bakery.", "image": ""},
    {"role": "customer", "role_name": "顾客", "text": "Thanks a lot! And can I pay by card?", "image": ""},
    {"role": "clerk", "role_name": "店员", "text": "Sure, we accept cards and mobile payment at checkout.", "image": ""}
  ],
  "key_vocab": [
    {"word": "aisle", "phonetic": "/aɪl/", "meaning": "（超市的）通道", "example": "It's in aisle three."},
    {"word": "on sale", "phonetic": "/ɒn seɪl/", "meaning": "特价；打折", "example": "This milk is on sale this week."},
    {"word": "checkout", "phonetic": "/ˈtʃekaʊt/", "meaning": "收银台；结账", "example": "You can pay at checkout."},
    {"word": "pay by card", "phonetic": "/peɪ baɪ kɑːrd/", "meaning": "刷卡支付", "example": "Can I pay by card?"}
  ],
  "grammar_points": [
    {"point": "How much is...?（问价格）", "explanation": "询问单数物品价格；复数用 How much are...?", "example": "How much is this milk?"},
    {"point": "Where can I find...?（问位置）", "explanation": "询问商品位置的地道表达。", "example": "Where can I find the bread?"}
  ],
  "practice": [
    {"type": "roleplay", "prompt": "你扮演顾客去超市买水果，向店员询问苹果价格和收银台位置。", "example": "How much are these apples? Where is the checkout?", "hints": ["How much are...?", "Where can I find...?"]},
    {"type": "speak", "prompt": "跟读 dialogue，注意 how much 和 can I 的连读。", "example": "朗读 dialogue 全篇", "hints": ["how much → /haʊ mʌtʃ/"]}
  ],
  "images": [
    "https://images.unsplash.com/photo-1542838132-92c53300491e?w=800",
    "https://images.unsplash.com/photo-1578916171728-46686eac8d58?w=800"
  ],
  "image_prompts": [
    "a bright modern supermarket aisle with shelves of groceries, a friendly clerk helping a customer choose milk, clean lighting, realistic illustration style",
    "a customer paying at a supermarket checkout counter with a card reader, shopping cart with bread and milk, fresh and colorful, realistic illustration style"
  ]
}`

const mockAirportContent = `{
  "unit": "Unit 7",
  "title": "At the Airport",
  "scene": "机场出行",
  "dialogue": [
    {"role": "clerk", "role_name": "地勤人员", "text": "Good morning, sir. May I see your passport and ticket, please?", "image": ""},
    {"role": "customer", "role_name": "旅客", "text": "Good morning. Here you are. I'm flying to New York.", "image": ""},
    {"role": "clerk", "role_name": "地勤人员", "text": "Thank you. Do you have any bags to check in?", "image": ""},
    {"role": "customer", "role_name": "旅客", "text": "Yes, one suitcase, please. Is it overweight?", "image": ""},
    {"role": "clerk", "role_name": "地勤人员", "text": "Let me see... No, it's 18 kilos, that's fine. Here is your boarding pass. Gate B22, boarding at 10:30.", "image": ""},
    {"role": "customer", "role_name": "旅客", "text": "Perfect. Which way is security?", "image": ""},
    {"role": "clerk", "role_name": "地勤人员", "text": "Go straight ahead and turn left. Have a nice flight!", "image": ""}
  ],
  "key_vocab": [
    {"word": "boarding pass", "phonetic": "/ˈbɔːrdɪŋ pæs/", "meaning": "登机牌", "example": "Here is your boarding pass."},
    {"word": "check in", "phonetic": "/tʃek ɪn/", "meaning": "办理登机手续", "example": "Do you have any bags to check in?"},
    {"word": "overweight", "phonetic": "/ˌoʊvərˈweɪt/", "meaning": "超重的", "example": "Is it overweight?"},
    {"word": "gate", "phonetic": "/ɡeɪt/", "meaning": "登机口", "example": "Gate B22, boarding at 10:30."}
  ],
  "grammar_points": [
    {"point": "May I...?（正式请求）", "explanation": "比 Can I 更正式礼貌的请求表达，机场/酒店高频。", "example": "May I see your passport?"},
    {"point": "have to + 动词原形", "explanation": "表示必须做某事。", "example": "I have to check in two hours early."}
  ],
  "practice": [
    {"type": "roleplay", "prompt": "你第一次坐飞机去机场，向地勤询问登机口位置和行李额度。", "example": "Excuse me, where is gate B22? How many bags can I bring?", "hints": ["May I...?", "Which way is...?"]},
    {"type": "speak", "prompt": "跟读 dialogue，注意数字与时间表达（10:30 → ten thirty）。", "example": "朗读 dialogue 全篇", "hints": ["18 kilos", "boarding at 10:30"]}
  ],
  "images": [
    "https://images.unsplash.com/photo-1436491865332-7a61a109cc05?w=800",
    "https://images.unsplash.com/photo-1529074963764-98f45c47344b?w=800"
  ],
  "image_prompts": [
    "a modern airport check-in counter with an airline clerk checking a passenger's passport and ticket, boarding pass in hand, bright airport hall, realistic illustration style",
    "an airport security area with a passenger walking toward gate signs, suitcase and boarding pass visible, blue and white airport interior, realistic illustration style"
  ]
}`
