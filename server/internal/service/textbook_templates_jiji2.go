package service

import "strconv"

// 本文件：冀教版（河北教育出版社）英语真实目录模板（第二批：四五六年级 + 八年级上下）
// 数据来源：课本站 kebenzhan.com 逐页目录（http 抓取核实）

func jiji4UpTemplate() []catalogTemplate {
	return jijiBuildV2("Grade4", [][2]string{
		{"Unit 1 The clothes we wear", "衣物"}, {"Unit 2 At home", "家庭"}, {"Unit 3 Let's go!", "出行"}, {"Unit 4 Shopping in the city", "购物"},
	}, [][]string{
		{"Skirt and trousers", "New and old", "Coat and scarf", "Shoes and socks", "Days of the week", "Betty's clothes"},
		{"Homework", "TV and phone", "In the bedroom", "Brush and wash", "Toys", "John and Jack"},
		{"At school", "Near and far", "In the city", "Cars and buses", "I'm lost!", "Lost in the city"},
		{"Let's go shopping!", "At the shop", "Cinema and park", "At the supermarket", "Shopping online", "Etta's teddy bear"},
	})
}

func jiji4DownTemplate() []catalogTemplate {
	return jijiBuildV2("Grade4", [][2]string{
		{"Unit 1 Hello again!", "问候"}, {"Unit 2 Days and months", "日期"}, {"Unit 3 All about me", "自我介绍"}, {"Unit 4 My favourites", "喜好"},
	}, [][]string{
		{"How are you?", "Is this your pencil?", "Where are they?", "How many books are there?", "Where is Danny?", "Where is Danny?"},
		{"Months of the year", "First,second,third", "When is it?", "Rain and sun", "How's the weather today?", "Mr. Moon's birthday"},
		{"How old are you?", "Are you short or tall?", "Where do you live?", "How do you go to school?", "What do you like to do?", "Maddy the monster"},
		{"My favourite colours", "My favourite clothes", "My favourite food", "My favourite subject", "My favourite school work", "The Diffos"},
	})
}

func jiji5UpTemplate() []catalogTemplate {
	return jijiBuildV2("Grade5", [][2]string{
		{"Unit 1 My Family", "家庭"}, {"Unit 2 My Country and English-speaking Countries", "国家"}, {"Unit 3 A Travel Plan", "旅行计划"}, {"Unit 4 Jenny and Danny Come to China", "中国之旅"},
	}, [][]string{
		{"Li Ming's big family", "What do they look like?", "What do they do?", "What do they like to do?", "Having fun together", "Maddy's family"},
		{"China", "Canada", "The U.S", "The U.K.", "Australia", "Maddy's family photos"},
		{"Beijing is great!", "May I go to Beijing?", "May I invite Danny and Jenny?", "How can we go to Beijing?", "The travel plan", "Billy Bee"},
		{"Meet Li Ming's family", "The Spring Festival is coming!", "What year is it?", "Special holidays in China", "What do we need for the trip?", "Year animals"},
	})
}

func jiji5DownTemplate() []catalogTemplate {
	return jijiBuildV2("Grade5", [][2]string{
		{"Unit 1 Going to Beijing", "去北京"}, {"Unit 2 In Beijing", "在北京"}, {"Unit 3 Writing Home", "写家书"}, {"Unit 4 Did You Have A Nice Trip?", "旅行归来"},
	}, [][]string{
		{"I Am Excited!", "What Are You Doing?", "Who is singing?", "Who is hungry?", "What are they doing?", "Danny is lost!"},
		{"Arriving in Beijing", "Tian'anmen Square", "The Palace Museum", "The Great Wall", "Shopping in Beijing", "Danny is lost!"},
		{"Let's buy postcards!", "Jenny writes a postcard", "Sending the postcards", "An email is fast", "Danny's email", "Little Zeke sends an email"},
		{"Li Ming comes home", "Jenny goes home", "Look at the photos!", "Gifts for Everyone", "An Email from Li Ming", "A Gift for Little Zeke"},
	})
}

func jiji6UpTemplate() []catalogTemplate {
	return jijiBuildV2("Grade6", [][2]string{
		{"Unit 1 Li Ming goes to Canada", "去加拿大"}, {"Unit 2 School in Canada", "加拿大校园"}, {"Unit 3 Winter in Canada", "加拿大冬天"}, {"Unit 4 Christmas", "圣诞节"},
	}, [][]string{
		{"At the airport", "Jenny's House", "Making breakfast", "Making dinner", "In the living room", "Baby Becky at home"},
		{"On the school bus", "Li Ming meets Jenny's class", "Mr. Wood teaches a science lesson", "How many are there?", "Always do your homework!", "Baby Becky at home"},
		{"Seasons", "Snow! It's winter!", "Winter fun", "A skating lesson", "I like all seasons!", "The snowman"},
		{"Christmas is coming!", "Christmas tree", "Christmas cards", "Special holidays in China", "It's Christmas morning!", "Maddy's Christmas"},
	})
}

func jiji6DownTemplate() []catalogTemplate {
	return jijiBuildV2("Grade6", [][2]string{
		{"Unit 1 Sports", "运动"}, {"Unit 2 Good health to you!", "健康"}, {"Unit 3 What will you do this summer?", "暑假计划"}, {"Unit 4 Li Ming comes home", "回家"},
	}, [][]string{
		{"Ping-pong and basketball", "At the sports shop", "Let's try!", "Did you have fun?", "A basketball game", "A famous football player"},
		{"Always have breakfast!", "Always brush your teeth!", "Eat more vegetables and fruit!", "Exercise", "Work hard!", "Helen Keller"},
		{"Summer is coming!", "Tomorrow we will play", "Jenny's summer holiday", "Li Ming's summer holiday", "Danny's summer holiday", "Three kites in the sky"},
		{"Buying gifts", "Looking at photos", "A party for Li Ming", "Surprise!", "Good-bye!", "Danny's surprise cake"},
	})
}

func jiji8UpTemplate() []catalogTemplate {
	return jijiBuildV2("Grade8", [][2]string{
		{"Unit 1 Me and My Class", "我与班级"}, {"Unit 2 My Favourite School Subject", "最喜欢的学科"}, {"Unit 3 Families celebrate together", "家庭庆祝"}, {"Unit 4 My neighbourhood", "我的社区"},
		{"Unit 5 My future", "我的未来"}, {"Unit 6 Go with transportation!", "交通工具"}, {"Unit 7 Enjoy your hobby", "爱好"}, {"Unit 8 Celebrating me!", "自我"},
	}, [][]string{
		{"Back to School", "Many Faces, One Picture", "Getting to Know You!", "Best Friends", "Meet Ms. Liu", "Jenny's week"},
		{"Don't be late for class!", "E-mail helps!", "I don't want to miss geography!", "Looking for Lisa", "Lily learns about China!", "Karen's hair stood up!"},
		{"I Love Autumn", "Happy Memories", "A Present For Li Ming", "Happy Thanksgiving!", "Presents from Canada!", "Li Ming's birthday"},
		{"The best neighbourhood", "No stopping!", "Eat a donut and turn right", "I like my neighbourhood", "People in my neighbourhood", "I need a map!"},
		{"I want to be a teacher!", "What will I be?", "What's your advice?", "Rich or poor? It doesn't matter!", "Our ambitions and dreams", "A famous friend?"},
		{"How do you travel?", "Trains go faster!", "Life on wheels", "Flying donuts", "Future transportation", "Clean cars?"},
		{"What's your hobby?", "Hobbies are fun!", "Danny's hobby", "What's Paul's hobby?", "Show and tell!", "The new club"},
		{"What Makes You Unique?", "Georgia plays basketball", "Be yourself!", "My dream", "I made it!", "Li Ming's report"},
	})
}

func jiji8DownTemplate() []catalogTemplate {
	return jijiBuildV2("Grade8", [][2]string{
		{"Unit 1 Spring is coming!", "春天"}, {"Unit 2 Plant a plant!", "植物"}, {"Unit 3 Animals are our friends", "动物朋友"}, {"Unit 4 The internet connects us", "互联网"},
		{"Unit 5 Buying and selling", "买卖"}, {"Unit 6 Be a champion!", "冠军"}, {"Unit 7 Know our world", "了解世界"}, {"Unit 8 Save our world", "保护世界"},
	}, [][]string{
		{"How's the weather?", "It's getting warmer!", "The sun is rising", "The spring city", "Babysitting on a Spring Day", "Stories about spring"},
		{"Planting trees", "Why are plants important?", "Gardening with Mary", "Make your garden grow!", "Amazing plants", "Danny's plant"},
		{"Danny's big scare", "Amazing animals", "The zoo is open", "The bear escaped!", "Save the tigers", "Friendship between animals"},
		{"How do you use the internet?", "A computer helps!", "Books or computers?", "Travel on the internet", "The internet - good or bad?", "An E-mail to Grandpa"},
		{"Raising money", "Cookies, please!", "Business English", "Mr.Liu's great idea", "How to push a product", "A cookie sale"},
		{"Don't fall, Danny!", "My favourite record", "2800 years of sports", "Modern Olympics", "The dream team", "Classroom Olympics"},
		{"Let's learn geography!", "The world is a big place", "Ring up or call?", "Body language", "A class of the world", "North America"},
		{"Let's clean up!", "Environment clubs", "Let's sort garbage!", "Connected to nature", "Garbage is interesting!"},
	})
}

// jijiBuildV2 冀教版模板生成（可变课数版）：每课 1 条目录项
func jijiBuildV2(level string, units [][2]string, lessonsPerUnit [][]string) []catalogTemplate {
	var out []catalogTemplate
	for ui, u := range units {
		lessons := lessonsPerUnit[ui]
		for i, title := range lessons {
			out = append(out, catalogTemplate{
				Level:  level,
				Unit:   u[0],
				Lesson: "Lesson " + strconv.Itoa(i+1),
				Title:  title,
				Scene:  u[1],
			})
		}
	}
	return out
}
