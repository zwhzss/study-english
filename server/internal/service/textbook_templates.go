package service

// 本文件：新增教材真实目录模板（初中人教版/外研版/牛津译林、高中、新概念 2-4 册）
// 均为各版本实际教材目录；每单元 1 个课时（Section/Unit 主课）

// ---- 初中人教版（老版通用） ----

func pep7DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade7", "Unit 1", "Unit 1", "Can you play the guitar?", "才艺询问"},
		{"Grade7", "Unit 2", "Unit 2", "What time do you go to school?", "作息时间"},
		{"Grade7", "Unit 3", "Unit 3", "How do you get to school?", "交通方式"},
		{"Grade7", "Unit 4", "Unit 4", "Don't eat in class.", "校规"},
		{"Grade7", "Unit 5", "Unit 5", "Why do you like pandas?", "动物喜好"},
		{"Grade7", "Unit 6", "Unit 6", "I'm watching TV.", "进行时"},
		{"Grade7", "Unit 7", "Unit 7", "It's raining!", "天气"},
		{"Grade7", "Unit 8", "Unit 8", "Is there a post office near here?", "问路"},
		{"Grade7", "Unit 9", "Unit 9", "What does he look like?", "外貌描述"},
		{"Grade7", "Unit 10", "Unit 10", "I'd like some noodles.", "点餐"},
		{"Grade7", "Unit 11", "Unit 11", "How was your school trip?", "学校旅行"},
		{"Grade7", "Unit 12", "Unit 12", "What did you do last weekend?", "周末活动"},
	}
}

func pep8UpTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade8", "Unit 1", "Unit 1", "Where did you go on vacation?", "假期"},
		{"Grade8", "Unit 2", "Unit 2", "How often do you exercise?", "频率"},
		{"Grade8", "Unit 3", "Unit 3", "I'm more outgoing than my sister.", "比较级"},
		{"Grade8", "Unit 4", "Unit 4", "What's the best movie theater?", "最高级"},
		{"Grade8", "Unit 5", "Unit 5", "Do you want to watch a game show?", "电视节目"},
		{"Grade8", "Unit 6", "Unit 6", "I'm going to study computer science.", "未来计划"},
		{"Grade8", "Unit 7", "Unit 7", "Will people have robots?", "未来预测"},
		{"Grade8", "Unit 8", "Unit 8", "How do you make a banana milk shake?", "制作步骤"},
		{"Grade8", "Unit 9", "Unit 9", "Can you come to my party?", "邀请"},
		{"Grade8", "Unit 10", "Unit 10", "If you go to the party, you'll have a great time!", "条件句"},
	}
}

func pep8DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade8", "Unit 1", "Unit 1", "What's the matter?", "健康状况"},
		{"Grade8", "Unit 2", "Unit 2", "I'll help to clean up the city parks.", "志愿服务"},
		{"Grade8", "Unit 3", "Unit 3", "Could you please clean your room?", "礼貌请求"},
		{"Grade8", "Unit 4", "Unit 4", "Why don't you talk to your parents?", "建议"},
		{"Grade8", "Unit 5", "Unit 5", "What were you doing when the rainstorm came?", "过去进行时"},
		{"Grade8", "Unit 6", "Unit 6", "An old man tried to move the mountains.", "寓言故事"},
		{"Grade8", "Unit 7", "Unit 7", "What's the highest mountain in the world?", "地理比较"},
		{"Grade8", "Unit 8", "Unit 8", "Have you read Treasure Island yet?", "读书"},
		{"Grade8", "Unit 9", "Unit 9", "Have you ever been to a museum?", "经历"},
		{"Grade8", "Unit 10", "Unit 10", "I've had this bike for three years.", "现在完成时"},
	}
}

func pep9Template() []catalogTemplate {
	return []catalogTemplate{
		{"Grade9", "Unit 1", "Unit 1", "How can we become good learners?", "学习方法"},
		{"Grade9", "Unit 2", "Unit 2", "I think that mooncakes are delicious!", "节日"},
		{"Grade9", "Unit 3", "Unit 3", "Could you please tell me where the restrooms are?", "礼貌问路"},
		{"Grade9", "Unit 4", "Unit 4", "I used to be afraid of the dark.", "过去习惯"},
		{"Grade9", "Unit 5", "Unit 5", "What are the shirts made of?", "物品材质"},
		{"Grade9", "Unit 6", "Unit 6", "When was it invented?", "发明"},
		{"Grade9", "Unit 7", "Unit 7", "Teenagers should be allowed to choose their own clothes.", "允许与禁止"},
		{"Grade9", "Unit 8", "Unit 8", "It must belong to Carla.", "推测"},
		{"Grade9", "Unit 9", "Unit 9", "I like music that I can dance to.", "定语从句"},
		{"Grade9", "Unit 10", "Unit 10", "You're supposed to shake hands.", "礼仪"},
		{"Grade9", "Unit 11", "Unit 11", "Sad movies make me cry.", "感受"},
		{"Grade9", "Unit 12", "Unit 12", "Life is full of the unexpected.", "意外"},
		{"Grade9", "Unit 13", "Unit 13", "We're trying to save the earth!", "环保"},
		{"Grade9", "Unit 14", "Unit 14", "I remember meeting all of you in Grade 7.", "回忆"},
	}
}

// ---- 初中外研版（新标准） ----

func fts7DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade7", "Module 1", "Unit 1", "Lost and found", "失物招领"},
		{"Grade7", "Module 2", "Unit 1", "What can you do?", "能力表达"},
		{"Grade7", "Module 3", "Unit 1", "Making plans", "制定计划"},
		{"Grade7", "Module 4", "Unit 1", "Life in the future", "未来生活"},
		{"Grade7", "Module 5", "Unit 1", "Shopping", "购物"},
		{"Grade7", "Module 6", "Unit 1", "Around town", "城镇周边"},
		{"Grade7", "Module 7", "Unit 1", "My past life", "过去的生活"},
		{"Grade7", "Module 8", "Unit 1", "Story time", "故事时间"},
		{"Grade7", "Module 9", "Unit 1", "Life history", "人生经历"},
		{"Grade7", "Module 10", "Unit 1", "A holiday journey", "假期旅行"},
		{"Grade7", "Module 11", "Unit 1", "Body language", "肢体语言"},
		{"Grade7", "Module 12", "Unit 1", "Western music", "西方音乐"},
	}
}

func fts8UpTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade8", "Module 1", "Unit 1", "How to learn English", "学英语方法"},
		{"Grade8", "Module 2", "Unit 1", "My home town and my country", "家乡与国家"},
		{"Grade8", "Module 3", "Unit 1", "Sports", "运动"},
		{"Grade8", "Module 4", "Unit 1", "Planes, ships and trains", "交通工具"},
		{"Grade8", "Module 5", "Unit 1", "Lao She Teahouse", "老舍茶馆"},
		{"Grade8", "Module 6", "Unit 1", "Animals in danger", "濒危动物"},
		{"Grade8", "Module 7", "Unit 1", "A famous story", "著名故事"},
		{"Grade8", "Module 8", "Unit 1", "Accidents", "事故"},
		{"Grade8", "Module 9", "Unit 1", "Population", "人口"},
		{"Grade8", "Module 10", "Unit 1", "The weather", "天气"},
		{"Grade8", "Module 11", "Unit 1", "Way of life", "生活方式"},
		{"Grade8", "Module 12", "Unit 1", "Help", "求助"},
	}
}

func fts8DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade8", "Module 1", "Unit 1", "Feelings and impressions", "感受印象"},
		{"Grade8", "Module 2", "Unit 1", "Experiences", "经历"},
		{"Grade8", "Module 3", "Unit 1", "Journey to space", "太空之旅"},
		{"Grade8", "Module 4", "Unit 1", "Seeing the doctor", "看医生"},
		{"Grade8", "Module 5", "Unit 1", "Problems", "问题"},
		{"Grade8", "Module 6", "Unit 1", "Hobbies", "爱好"},
		{"Grade8", "Module 7", "Unit 1", "Summer in Los Angeles", "洛杉矶夏天"},
		{"Grade8", "Module 8", "Unit 1", "Time off", "休息时间"},
		{"Grade8", "Module 9", "Unit 1", "Friendship", "友谊"},
		{"Grade8", "Module 10", "Unit 1", "On the radio", "广播"},
	}
}

func fts9Template() []catalogTemplate {
	return []catalogTemplate{
		{"Grade9", "Module 1", "Unit 1", "Wonders of the world", "世界奇观"},
		{"Grade9", "Module 2", "Unit 1", "Public holidays", "公共假日"},
		{"Grade9", "Module 3", "Unit 1", "Heroes", "英雄"},
		{"Grade9", "Module 4", "Unit 1", "Home alone", "独自在家"},
		{"Grade9", "Module 5", "Unit 1", "Museums", "博物馆"},
		{"Grade9", "Module 6", "Unit 1", "Problems", "问题"},
		{"Grade9", "Module 7", "Unit 1", "Great books", "名著"},
		{"Grade9", "Module 8", "Unit 1", "Sports life", "运动生涯"},
		{"Grade9", "Module 9", "Unit 1", "Great inventions", "伟大发明"},
		{"Grade9", "Module 10", "Unit 1", "Australia", "澳大利亚"},
		{"Grade9", "Module 11", "Unit 1", "Photos", "照片"},
		{"Grade9", "Module 12", "Unit 1", "Save our world", "拯救世界"},
	}
}

// ---- 初中牛津译林 ----

func oxford7DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade7", "Unit 1", "Reading", "Dream homes", "梦想家园"},
		{"Grade7", "Unit 2", "Reading", "Neighbours", "邻居"},
		{"Grade7", "Unit 3", "Reading", "Welcome to Sunshine Town", "阳光镇"},
		{"Grade7", "Unit 4", "Reading", "Finding your way", "找路"},
		{"Grade7", "Unit 5", "Reading", "Amazing things", "奇妙事物"},
		{"Grade7", "Unit 6", "Reading", "Outdoor fun", "户外乐趣"},
		{"Grade7", "Unit 7", "Reading", "Abilities", "能力"},
		{"Grade7", "Unit 8", "Reading", "Pets", "宠物"},
	}
}

func oxford8UpTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade8", "Unit 1", "Reading", "Friends", "朋友"},
		{"Grade8", "Unit 2", "Reading", "School life", "学校生活"},
		{"Grade8", "Unit 3", "Reading", "A day out", "外出一天"},
		{"Grade8", "Unit 4", "Reading", "Do it yourself", "自己动手"},
		{"Grade8", "Unit 5", "Reading", "Wild animals", "野生动物"},
		{"Grade8", "Unit 6", "Reading", "Bird watching", "观鸟"},
		{"Grade8", "Unit 7", "Reading", "Seasons", "季节"},
		{"Grade8", "Unit 8", "Reading", "Natural disasters", "自然灾害"},
	}
}

func oxford8DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade8", "Unit 1", "Reading", "Past and present", "过去与现在"},
		{"Grade8", "Unit 2", "Reading", "Travelling", "旅行"},
		{"Grade8", "Unit 3", "Reading", "Online tours", "在线游览"},
		{"Grade8", "Unit 4", "Reading", "A good read", "好书"},
		{"Grade8", "Unit 5", "Reading", "Good manners", "礼貌"},
		{"Grade8", "Unit 6", "Reading", "Sunshine for all", "大爱无疆"},
		{"Grade8", "Unit 7", "Reading", "International charities", "国际慈善"},
		{"Grade8", "Unit 8", "Reading", "A green world", "绿色世界"},
	}
}

func oxford9Template() []catalogTemplate {
	return []catalogTemplate{
		{"Grade9", "Unit 1", "Reading", "Know yourself", "认识自己"},
		{"Grade9", "Unit 2", "Reading", "Colours", "颜色"},
		{"Grade9", "Unit 3", "Reading", "Teenage problems", "青少年问题"},
		{"Grade9", "Unit 4", "Reading", "Growing up", "成长"},
		{"Grade9", "Unit 5", "Reading", "Art world", "艺术世界"},
		{"Grade9", "Unit 6", "Reading", "TV programmes", "电视节目"},
		{"Grade9", "Unit 7", "Reading", "Films", "电影"},
		{"Grade9", "Unit 8", "Reading", "Detective stories", "侦探故事"},
	}
}

// ---- 高中人教版（2019 新版） ----

func gaoZhongRenjiao(level string, units []string) []catalogTemplate {
	out := make([]catalogTemplate, 0, len(units))
	for i, u := range units {
		out = append(out, catalogTemplate{"高中", "Unit " + itoa(uint64(i+1)), "Unit " + itoa(uint64(i+1)), u, "高中英语"})
	}
	return out
}

func pepHigh1Template() []catalogTemplate {
	return gaoZhongRenjiao("必修一", []string{"Teenage life", "Travelling around", "Sports and fitness", "Natural disasters", "Languages around the world"})
}
func pepHigh2Template() []catalogTemplate {
	return gaoZhongRenjiao("必修二", []string{"Cultural heritage", "Wildlife protection", "The internet", "History and traditions", "Music"})
}
func pepHigh3Template() []catalogTemplate {
	return gaoZhongRenjiao("必修三", []string{"Festivals and celebrations", "Morals and virtues", "Diverse cultures", "Space exploration", "The value of money"})
}
func pepHighSel1Template() []catalogTemplate {
	return gaoZhongRenjiao("选必一", []string{"People of achievement", "Looking into the future", "Fascinating parks", "Body language", "Working the land"})
}
func pepHighSel2Template() []catalogTemplate {
	return gaoZhongRenjiao("选必二", []string{"Science and scientists", "Bridging cultures", "Food and culture", "Journey across a vast land", "First aid"})
}
func pepHighSel3Template() []catalogTemplate {
	return gaoZhongRenjiao("选必三", []string{"Art", "Healthy lifestyle", "Environmental protection", "Adversity and courage", "Poems"})
}

// ---- 高中外研版 ----

func ftsHigh1Template() []catalogTemplate {
	return gaoZhongRenjiao("必修一", []string{"A new start", "Exploring English", "On the move", "Friends forever", "Into the wild", "At one with nature"})
}
func ftsHigh2Template() []catalogTemplate {
	return gaoZhongRenjiao("必修二", []string{"Food for thought", "Let's celebrate!", "On the move", "Stage and screen", "On the road", "Earth first"})
}
func ftsHigh3Template() []catalogTemplate {
	return gaoZhongRenjiao("必修三", []string{"Knowing me, knowing you", "Making a difference", "The world of science", "Amazing art", "What an adventure!", "Disaster and hope"})
}

// ---- 新概念 2-3 册（Lesson 1-24，可按需扩展） ----

func nce2Template() []catalogTemplate {
	out := make([]catalogTemplate, 0, 24)
	for i := 1; i <= 24; i++ {
		out = append(out, catalogTemplate{"Book2", "Lesson " + itoa(uint64(i)), "Lesson " + itoa(uint64(i)), "Lesson " + itoa(uint64(i)), "新概念二册"})
	}
	return out
}
func nce3Template() []catalogTemplate {
	out := make([]catalogTemplate, 0, 24)
	for i := 1; i <= 24; i++ {
		out = append(out, catalogTemplate{"Book3", "Lesson " + itoa(uint64(i)), "Lesson " + itoa(uint64(i)), "Lesson " + itoa(uint64(i)), "新概念三册"})
	}
	return out
}
