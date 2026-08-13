package service

import (
	"strconv"
)

// 本文件：冀教版（河北教育出版社）英语真实目录模板
// 数据来源：课本站 kebenzhan.com 逐页目录（已核实）；每单元 6 课，每课 1 条目录项
// 已核实完整：小学三年级下册、七年级上册、七年级下册、九年级全一册

// ---- 冀教版小学英语三年级下册（4 单元 × 6 课） ----

func jiji3DownTemplate() []catalogTemplate {
	units := []struct {
		unit    string
		scene   string
		lessons [6]string
	}{
		{"Unit 1 Animals on the farm", "农场动物", [6]string{"On the farm", "Cats And Dogs", "Fish And Birds", "Horses And Rabbits", "Where?", "Can I Help You?"}},
		{"Unit 2 Animals at the zoo", "动物园动物", [6]string{"At The Zoo", "Tigers and Bears", "How Many?", "Where Do They Live?", "What Do They Eat?", "The Clever Monkey"}},
		{"Unit 3 Food and meals", "食物与三餐", [6]string{"I'm hungry!", "Would You Like Some Soup?", "What's Your Favourite Food?", "Breakfast, Lunch And Dinner", "What's For Breakfast?", "The Magic Stone"}},
		{"Unit 4 Food and restaurants", "餐厅饮食", [6]string{"I Like Fruit!", "Hamburgers and hot dogs", "In the restaurant", "How Much Is It?", "How much are they?", "A little monkey"}},
	}
	return jijiBuild("Grade3", units)
}

// ---- 冀教版七年级上册（8 单元 × 6 课） ----

func jiji7UpTemplate() []catalogTemplate {
	units := []struct {
		unit    string
		scene   string
		lessons [6]string
	}{
		{"Unit 1 School and friends", "学校与朋友", [6]string{"Hello!", "Teachers and students", "Welcome to our school", "What is it?", "May I have a book?", "Things for school"}},
		{"Unit 2 Colours and clothes", "颜色与衣物", [6]string{"Jenny's new skirt", "Danny's favourite colour", "Whose coat is this?", "Clothes for a cold day", "Clothes around the world", "Let's go shopping!"}},
		{"Unit 3 Body parts and feelings", "身体与感受", [6]string{"Body parts", "Colours and feelings", "Tall or short", "Happy or sad", "Seeing a doctor", "We all look different!"}},
		{"Unit 4 Food and restaurants", "食物与餐厅", [6]string{"Time for breakfast!", "I like the supermarket!", "At the market", "In the restaurant", "The corner store", "Eat good food!"}},
		{"Unit 5 Family and home", "家庭", [6]string{"Jenny's family", "Li Ming's family", "Danny at home", "A family picnic", "A birthday card", "Grandma's birthday party"}},
		{"Unit 6 Let's go!", "出行游玩", [6]string{"Let's go to the bookstore!", "At the supermarket", "Let's go to the zoo!", "On the farm", "Let's go to the museum!", "Let's go to the movie theatre!"}},
		{"Unit 7 Days and months", "日子与月份", [6]string{"Seasons and weather", "Nick's busy month", "A class calendar", "When is your birthday?", "Holidays", "Happy birthdays!"}},
		{"Unit 8 Countries around the world", "世界各国", [6]string{"Directions", "Jack's goodbye party", "China", "Canada and the U.S.", "The U.K. and Australia", "English-speaking countries"}},
	}
	return jijiBuild("Grade7", units)
}

// ---- 冀教版七年级下册（8 单元 × 6 课） ----

func jiji7DownTemplate() []catalogTemplate {
	units := []struct {
		unit    string
		scene   string
		lessons [6]string
	}{
		{"Unit 1 A Trip to the Silk Road", "丝绸之路旅行", [6]string{"A trip to China", "Meet you in Beijing", "A visit to Xi'an", "A visit to Lanzhou", "Another stop along the Silk Road", "Jenny's Diary"}},
		{"Unit 2 It's show time!", "才艺展示", [6]string{"What's your project about?", "Marco Polo and the Silk Road", "Danny's school project", "Music and dance", "Food in China", "A blog about the Silk Road"}},
		{"Unit 3 School life", "校园生活", [6]string{"How is school going?", "Jenny's school life", "Making a difference", "We are with you!", "School science fair", "Teaching in China"}},
		{"Unit 4 After-school activities", "课外活动", [6]string{"A dinner date", "Join our club!", "What is your club type?", "Big plans for the weekend", "A weekend with Grandma", "How was your weekend?"}},
		{"Unit 5 I love learning English!", "英语学习", [6]string{"A phone friend", "Online phone calls", "Amazing English", "How do I learn English?", "A door to the world", "Writing an E-mail in English"}},
		{"Unit 6 Seasons", "四季", [6]string{"What strange weather!", "I can't wait for winter!", "Kim's favourite season", "Steven's report", "Surfing in Sydney", "Spring in China"}},
		{"Unit 7 Sports and good health", "运动与健康", [6]string{"You are what you eat!", "Stay healthy!", "Danny's report", "Move your body", "Were people healthy then?", "Know yourself"}},
		{"Unit 8 Summer holiday is coming!", "暑假计划", [6]string{"Have a good summer!", "Volunteering in summer", "Baseball season", "Get ready for summer holiday!", "Summer plans", "Li Ming's Summer Holiday"}},
	}
	return jijiBuild("Grade7", units)
}

// ---- 冀教版九年级全一册（10 单元 × 6 课，个别缺课按已核实部分） ----

func jiji9Template() []catalogTemplate {
	units := []struct {
		unit    string
		scene   string
		lessons [6]string
	}{
		{"Unit 1 Stay Healthy", "健康生活", [6]string{"What's Wrong, Danny?", "A visit to the Dentist", "Good Food, Good Health", "Don't Smoke, Please!", "Jane's Lucky Life", "Stay Away From The Hospital"}},
		{"Unit 2 Great People", "伟人", [6]string{"What is the meaning of life?", "A Universe of Thought", "China's Most Famous Farmer", "Touch The World", "To China, With Love", "Great People Review"}},
		{"Unit 3 Safety", "安全", [6]string{"Be careful, Danny!", "Accidents happen", "My helmet saved my life!", "How safe is your home?", "Staying safe in an earthquake", "Never Catch A Dinosaur"}},
		{"Unit 4 Stories And Poems", "故事与诗歌", [6]string{"A Story or a poem?", "Say it in five", "The Fable of the woodcutter", "The Giant(II)", "Writing a poem", "Stories And Poems Review"}},
		{"Unit 5 Look into science", "探索科学", [6]string{"Let's do an experiment!", "Keep the candle burning", "Planet Danny", "The Study Of Living things", "DNA-The Story Of You", "Science affects us"}},
		{"Unit 6 Movies and theatre", "电影与戏剧", [6]string{"A movie or a play", "Moving pictures", "The Fisherman and the Goldfish(I)", "The Fisherman and the Goldfish(II)", "Theatres are fun!", "Making plays is fun"}},
		{"Unit 7 Work For Peace", "和平", [6]string{"Don't Fight!", "Making school a better place", "The dove and the olive branch", "The UN - The power of words", "Jenny's Good Advice", "Peace at last"}},
		{"Unit 8 Culture Shapes Us", "文化", [6]string{"A visit to Chinatown", "Popular sayings", "Different manners", "Home to many cultures", "Good manners", "Supper with the Bradshaws"}},
		{"Unit 9 Communication", "沟通", [6]string{"Get along with others", "Tips For Good communication", "What Could Be A Wrong?", "The power of a smile", "Working in groups", "How embarrassing!"}},
		{"Unit 10 Get ready for the future", "面向未来", [6]string{"Look into the future", "Manage your time", "Best wishes", "Ms.Liu's speech", "Keep Your Choices Open", "Get a good education"}},
	}
	return jijiBuild("Grade9", units)
}

// jijiBuild 冀教版模板生成：每课 1 条目录项（Level=年级，Unit=单元名+标题，Lesson=Lesson N，Title=课文名）
func jijiBuild(level string, units []struct {
	unit    string
	scene   string
	lessons [6]string
}) []catalogTemplate {
	var out []catalogTemplate
	for _, u := range units {
		for i, title := range u.lessons {
			out = append(out, catalogTemplate{
				Level:  level,
				Unit:   u.unit,
				Lesson: "Lesson " + strconv.Itoa(i+1),
				Title:  title,
				Scene:  u.scene,
			})
		}
	}
	return out
}
