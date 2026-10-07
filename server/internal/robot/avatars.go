package robot

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// 按性别分池:男性头像取自 uploads/robot_man/,女性取自 uploads/robot/。
var avatarPoolFemale []string
var avatarPoolMale []string

// URL 片段,用于判断某头像 URL 属于哪个性别池(迁移幂等判断)。
const (
	femalePathTag = "/static/robot/"
	malePathTag   = "/static/robot_man/"
)

// InitAvatarPool 扫描 uploadDir 下 robot/(女) 与 robot_man/(男),构建两个头像 URL 池。
// 在 main 启动时调用一次,早于 robot.Start。
func InitAvatarPool(uploadDir, publicBaseURL string) {
	base := strings.TrimRight(publicBaseURL, "/")
	avatarPoolFemale = scanAvatars(filepath.Join(uploadDir, "robot"), base+femalePathTag)
	avatarPoolMale = scanAvatars(filepath.Join(uploadDir, "robot_man"), base+malePathTag)
}

func scanAvatars(dir, urlBase string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var pool []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") {
			pool = append(pool, urlBase+e.Name())
		}
	}
	return pool
}

// randomAvatarByGender 按性别返回随机头像 URL。gender:1=男 2=女。
// 对应性别池为空时回退到另一池,保证尽量有头像。
func randomAvatarByGender(gender int8) string {
	pool := avatarPoolFemale
	if gender == 1 {
		pool = avatarPoolMale
	}
	if len(pool) == 0 { // 回退:对应池没素材时用另一池
		if gender == 1 {
			pool = avatarPoolFemale
		} else {
			pool = avatarPoolMale
		}
	}
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))]
}

// genderPathTag 返回该性别头像 URL 应包含的目录片段,用于迁移幂等判断。
func genderPathTag(gender int8) string {
	if gender == 1 {
		return malePathTag
	}
	return femalePathTag
}
