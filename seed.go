package main

import (
	"strings"
	"time"

	"nexttodo/locales"
)

// defaultCategories are the categories a first launch starts with, named
// in lang: they are the user's to rename, recolor, reorder and delete.
func defaultCategories(lang string) []*Category {
	tr := locales.In(lang)
	return []*Category{
		{ID: defaultCatIDs[0], Name: tr.CatFunded(), Note: tr.CatFundedHint(), Color: 0, Icon: "flask"},
		{ID: defaultCatIDs[1], Name: tr.CatIndustry(), Note: tr.CatIndustryHint(), Color: 1, Icon: "building"},
		{ID: defaultCatIDs[2], Name: tr.CatIndependent(), Note: tr.CatIndependentHint(), Color: 2, Icon: "bulb"},
		{ID: defaultCatIDs[3], Name: tr.CatService(), Note: tr.CatServiceHint(), Color: 4, Icon: "book"},
	}
}

// defaultKinds are the tags events start with, in the language lang.
func defaultKinds(lang string) []*Kind {
	tr := locales.In(lang)
	return []*Kind{
		{ID: defaultKindIDs[0], Name: tr.KindMeeting(), Color: 0},
		{ID: defaultKindIDs[1], Name: tr.KindLab(), Color: 2},
		{ID: defaultKindIDs[2], Name: tr.KindSeminar(), Color: 3},
		{ID: defaultKindIDs[3], Name: tr.KindFocus(), Color: 5},
		{ID: defaultKindIDs[4], Name: tr.KindOther(), Color: 6},
	}
}

// sampleData is the almanac of an imagined researcher's season, around
// now, in English or Chinese, for a first launch to show what the app
// keeps.
func sampleData(now time.Time, lang string) *Data {
	zh := strings.HasPrefix(lang, "zh")
	s := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}
	today := day(now)
	d := newData()
	d.Categories, d.Kinds = defaultCategories(lang), defaultKinds(lang)
	funded, industry, independent, service := defaultCatIDs[0], defaultCatIDs[1], defaultCatIDs[2], defaultCatIDs[3]
	created := now.Add(-90 * 24 * time.Hour)

	dir := func(title, summary string, primary bool) *Direction {
		x := &Direction{ID: newID(), Title: title, Summary: summary, Primary: primary, Created: created}
		d.Directions = append(d.Directions, x)
		return x
	}
	multimodal := dir(s("Multimodal representation learning", "多模态表征学习"),
		s("How vision and language come to share one space, with as little supervision as possible.",
			"视觉与语言如何以尽可能少的监督，共享同一个表征空间。"), true)
	edge := dir(s("Efficient inference on the edge", "端侧高效推理"),
		s("Making large models small enough for the devices people carry.", "让大模型小到足以运行在人们随身携带的设备上。"), false)
	science := dir(s("AI for science", "科学智能"),
		s("Learning from the data of the lab: spectra, cells and crystals.", "从实验室的数据中学习：光谱、细胞与晶体。"), false)

	proj := func(p *Project) *Project {
		p.ID, p.Created = newID(), created
		d.Projects = append(d.Projects, p)
		return p
	}
	nsfc := proj(&Project{
		Title: s("Weakly supervised cross-modal alignment", "弱监督跨模态对齐方法研究"), Code: "NSFC 62376118",
		Category: funded, Status: StatusActive, Direction: multimodal.ID, Lead: s("Principal investigator", "项目负责人"),
		Funder: s("National Natural Science Foundation of China", "国家自然科学基金委员会"),
		Start:  time.Date(2025, 1, 1, 0, 0, 0, 0, now.Location()), End: time.Date(2028, 12, 31, 0, 0, 0, 0, now.Location()),
		Summary: s("Aligning images and text from noisy web pairs, with guarantees on what the alignment preserves.", "从带噪声的网络图文对中学习对齐，并刻画对齐所保留的语义结构。"),
		Journal: []Entry{
			{ID: newID(), Date: now.Add(-26 * time.Hour), Text: s("Sigmoid loss beats softmax at batch 4k on our noisy split; gap closes at 32k. Worth a paragraph in the revision.", "在噪声划分上，batch 4k 时 sigmoid 损失优于 softmax；到 32k 时差距消失。值得在修改稿中写一段。")},
			{ID: newID(), Date: now.Add(-5 * 24 * time.Hour), Text: s("Meeting with Dr. Zhou: share the curated 2M-pair subset with the spectroscopy group.", "与周老师讨论：将清洗后的 200 万图文对子集共享给光谱组。")},
		},
	})
	hangxin := proj(&Project{
		Title: s("On-device speech model compression", "端侧语音模型压缩"), Code: "HX-2026-031",
		Category: industry, Status: StatusActive, Direction: edge.ID, Lead: s("Technical lead", "技术负责人"),
		Funder: s("Hangxin Technology", "杭芯科技"), Start: addDays(today, -120), End: addDays(today, 38),
		Summary: s("A streaming recognizer under 40 MB that keeps word error rate within 5% of the server model.", "将流式识别模型压缩到 40 MB 以内，词错率与服务端模型相差不超过 5%。"),
	})
	survey := proj(&Project{
		Title:    s("Survey: retrieval-augmented reasoning", "综述：检索增强推理"),
		Category: independent, Status: StatusPlanning, Direction: multimodal.ID, End: addDays(today, 75),
		Summary: s("Mapping how retrieval changes what language models can reason about.", "梳理检索如何改变语言模型的推理能力。"),
	})
	reviewing := proj(&Project{
		Title: s("CVPR 2027 reviewing", "CVPR 2027 审稿"), Category: service, Status: StatusActive, End: addDays(today, 18),
		Summary: s("Six papers in the area of vision and language.", "视觉与语言方向，共六篇。"),
	})
	proj(&Project{
		Title: s("Spectral foundation model for materials", "材料光谱基础模型"), Code: "2024ZD0101",
		Category: funded, Status: StatusPaused, Direction: science.ID, Funder: s("Provincial Department of Science and Technology", "省科技厅"),
		Summary: s("Waiting on the new spectrometer before collecting the second corpus.", "等待新光谱仪到位后采集第二批数据。"),
	})
	proj(&Project{
		Title: s("Graduate course: deep learning", "研究生课程：深度学习"), Category: service, Status: StatusDone,
		Summary: s("Spring term, 64 students. Slides and notes are in the course folder.", "春季学期，64 名学生，课件与讲义已归档。"),
	})

	task := func(title string, p *Project, due int, prio int, starred bool) *Task {
		t := &Task{ID: newID(), Title: title, Priority: prio, Starred: starred, Created: now.Add(-48 * time.Hour)}
		if p != nil {
			t.Project = p.ID
		}
		if due != noDue {
			t.Due = addDays(today, due)
		}
		d.Tasks = append(d.Tasks, t)
		return t
	}
	task(s("Draft the ablation section of the TPAMI revision", "撰写 TPAMI 修改稿的消融实验部分"), nsfc, 0, 3, true)
	task(s("Re-run the CLIP baselines with new seeds", "用新的随机种子重跑 CLIP 基线"), nsfc, 2, 2, false)
	task(s("Annual progress report to the NSFC", "提交基金年度进展报告"), nsfc, 9, 3, true)
	task(s("Reply to reviewer 2", "回复审稿人 2 的意见"), nsfc, -1, 2, false).setDone(true, now.Add(-20*time.Hour))
	task(s("Quantize the encoder to INT4 and measure WER", "将编码器量化至 INT4 并测量词错率"), hangxin, 1, 2, false)
	task(s("Prepare the mid-term demo for Hangxin", "准备杭芯项目中期演示"), hangxin, 6, 2, true)
	task(s("Sign the data-sharing agreement", "签署数据共享协议"), hangxin, -2, 3, false)
	task(s("Collect sixty core papers", "收集六十篇核心论文"), survey, noDue, 1, false).MyDay = today
	task(s("Outline the taxonomy", "拟定综述的分类框架"), survey, 12, 0, false)
	task(s("Review paper #1043", "审阅 1043 号论文"), reviewing, 4, 2, false)
	task(s("Review paper #2211", "审阅 2211 号论文"), reviewing, 4, 1, false)
	task(s("Read: sigmoid loss for language–image pre-training", "精读：图文预训练中的 sigmoid 损失"), nsfc, noDue, 0, false).MyDay = today
	task(s("Book flights for the December workshop", "预订十二月研讨会的机票"), nil, 3, 1, false).MyDay = today
	vpn := task(s("Renew the library VPN", "续期图书馆 VPN"), nil, noDue, 0, false)
	vpn.MyDay = today
	vpn.setDone(true, now.Add(-time.Hour))

	event := func(title string, k KindID, p *Project, dayOffset, h, m, minutes int, place string) *Event {
		start := at(addDays(today, dayOffset), h, m)
		e := &Event{ID: newID(), Title: title, Kind: k, Location: place, Start: start, End: start.Add(time.Duration(minutes) * time.Minute)}
		if p != nil {
			e.Project = p.ID
		}
		d.Events = append(d.Events, e)
		return e
	}
	event(s("Group meeting", "组会"), "meeting", nil, 0, 9, 30, 60, s("Room 402", "402 会议室")).Repeat = RepeatWeekly
	event(s("GPU cluster: alignment runs", "GPU 集群：对齐实验"), "lab", nsfc, 0, 14, 0, 180, s("Cluster B", "B 区集群"))
	event(s("Writing: TPAMI revision", "写作：TPAMI 修改稿"), "focus", nsfc, 0, 19, 30, 90, "")
	event(s("Seminar: world models", "学术报告：世界模型"), "seminar", nil, -1, 10, 0, 60, s("Lecture hall 2", "第二报告厅"))
	event(s("Hangxin weekly sync", "杭芯周例会"), "meeting", hangxin, 1, 10, 0, 90, s("Video call", "腾讯会议")).Repeat = RepeatWeekly
	event(s("Reading group: diffusion", "读书会：扩散模型"), "seminar", nil, 1, 15, 0, 90, s("Library 3F", "图书馆三楼")).Repeat = RepeatBiweekly
	event(s("Deep work: survey outline", "专注：综述提纲"), "focus", survey, 2, 9, 0, 180, "")
	event(s("Advising: Lin's proposal", "指导：林同学开题"), "meeting", nil, 2, 14, 0, 60, s("Office", "办公室"))
	event(s("Data collection with Dr. Zhou", "与周老师采集数据"), "lab", nsfc, 3, 13, 30, 240, s("Spectroscopy lab", "光谱实验室"))
	event(s("Department colloquium", "院系学术讲座"), "seminar", nil, 4, 16, 0, 60, s("Lecture hall 1", "第一报告厅"))
	event(s("Review papers", "集中审稿"), "focus", reviewing, 5, 10, 0, 120, "")
	retreat := at(addDays(today, 3), 0, 0)
	d.Events = append(d.Events, &Event{ID: newID(), Title: s("Faculty retreat", "系务研讨日"), Kind: "other", AllDay: true, Start: retreat, End: addDays(retreat, 1)})
	return d
}

const noDue = 1 << 20
