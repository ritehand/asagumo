package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	bot "github.com/1l0/asagumo"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

const (
	TargetChannelID = "1473375081672736838"
)

var (
	districtRolePattern = regexp.MustCompile(`^[0-9]+区$`)
	prefectures         = []string{
		"北海道", "青森県", "岩手県", "宮城県", "秋田県", "山形県", "福島県",
		"茨城県", "栃木県", "群馬県", "埼玉県", "千葉県", "神奈川県", "山梨県",
		"東京都", "新潟県", "富山県", "石川県", "福井県", "長野県", "岐阜県",
		"静岡県", "愛知県", "三重県", "滋賀県", "京都府", "大阪府", "兵庫県",
		"奈良県", "和歌山県", "鳥取県", "島根県", "岡山県", "広島県", "山口県",
		"徳島県", "香川県", "愛媛県", "高知県", "福岡県", "佐賀県", "長崎県",
		"熊本県", "大分県", "宮崎県", "鹿児島県", "沖縄県",
	}
)

func init() {
	log.SetFlags(0)
}

func main() {
	db, err := bot.InitDB()
	if err != nil {
		log.Fatalln("Failed to initialize database:", err)
	}
	defer db.Close()

	s, err := discordgo.New("Bot " + bot.Token)
	if err != nil {
		log.Fatalln(err)
	}

	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionMessageComponent {
			handleDistrictButton(s, i, db)
		}
	})

	if err := s.Open(); err != nil {
		log.Fatalln(err)
	}
	defer s.Close()

	log.Println("Bot is running. Setting up UI...")
	if err := setupUI(s); err != nil {
		log.Printf("Failed to setup UI: %v", err)
	}

	select {}
}

func setupUI(s *discordgo.Session) error {
	// Check if UI already exists in the channel
	messages, err := s.ChannelMessages(TargetChannelID, 50, "", "", "")
	if err == nil {
		for _, m := range messages {
			if m.Author.ID == s.State.User.ID {
				for _, row := range m.Components {
					if actionsRow, ok := row.(*discordgo.ActionsRow); ok {
						for _, comp := range actionsRow.Components {
							if btn, ok := comp.(*discordgo.Button); ok {
								if strings.HasPrefix(btn.CustomID, "dist_") {
									log.Println("UI already exists, skipping construction.")
									return nil
								}
							}
						}
					}
				}
			}
		}
	}

	// Discord allows max 5 rows of 5 buttons per message.
	// Total 30 buttons needed -> 25 in first message, 5 in second message.

	createMessage := func(start, end int) *discordgo.MessageSend {
		var rows []discordgo.MessageComponent
		var currentRow []discordgo.MessageComponent

		for i := start; i <= end; i++ {
			btn := discordgo.Button{
				Label:    fmt.Sprintf("%d区", i),
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("dist_%d", i),
			}
			currentRow = append(currentRow, btn)

			if len(currentRow) == 5 {
				rows = append(rows, discordgo.ActionsRow{Components: currentRow})
				currentRow = nil
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, discordgo.ActionsRow{Components: currentRow})
		}

		return &discordgo.MessageSend{
			Content:    fmt.Sprintf("選挙区（%d区〜%d区）を選択してください：", start, end),
			Components: rows,
		}
	}

	// For simplicity, we just send new messages.
	// In a real app, you might want to edit existing ones if found.
	_, err = s.ChannelMessageSendComplex(TargetChannelID, createMessage(1, 25))
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSendComplex(TargetChannelID, createMessage(26, 30))
	return err
}

func handleDistrictButton(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB) {
	customID := i.MessageComponentData().CustomID
	if !strings.HasPrefix(customID, "dist_") {
		return
	}

	targetDistNum, _ := strconv.Atoi(strings.TrimPrefix(customID, "dist_"))
	userID := i.Member.User.ID

	// Get guild roles to map IDs to names
	roles, err := s.GuildRoles(i.GuildID)
	if err != nil {
		sendEphemeral(s, i, "エラー：サーバー情報の取得に失敗しました。")
		return
	}
	roleMap := make(map[string]string) // ID -> Name
	for _, r := range roles {
		roleMap[r.ID] = r.Name
	}

	// Identify user's prefecture role
	var userPref string
	for _, roleID := range i.Member.Roles {
		roleName := roleMap[roleID]
		for _, p := range prefectures {
			if roleName == p {
				userPref = p
				break
			}
		}
		if userPref != "" {
			break
		}
	}

	if userPref == "" {
		sendEphemeral(s, i, "都道府県ロールが付与されていません。")
		return
	}

	// Check district count for this prefecture
	var districtCount int
	err = db.QueryRow("SELECT district_count FROM prefectures WHERE name = ?", userPref).Scan(&districtCount)
	if err != nil {
		sendEphemeral(s, i, fmt.Sprintf("エラー：データベースから%sの情報を取得できませんでした。", userPref))
		return
	}

	if targetDistNum > districtCount {
		sendEphemeral(s, i, fmt.Sprintf("%sには%d区までしか存在しません（%d区を選択）。", userPref, districtCount, targetDistNum))
		return
	}

	// Update roles: Remove current district roles, Add new one
	targetRoleName := fmt.Sprintf("%d区", targetDistNum)
	var targetRoleID string
	for _, r := range roles {
		if r.Name == targetRoleName {
			targetRoleID = r.ID
			break
		}
	}

	if targetRoleID == "" {
		sendEphemeral(s, i, fmt.Sprintf("エラー：%sのロールが見つかりませんでした。", targetRoleName))
		return
	}

	// Remove existing district roles
	for _, roleID := range i.Member.Roles {
		roleName := roleMap[roleID]
		if districtRolePattern.MatchString(roleName) {
			s.GuildMemberRoleRemove(i.GuildID, userID, roleID)
		}
	}

	// Add new district role
	err = s.GuildMemberRoleAdd(i.GuildID, userID, targetRoleID)
	if err != nil {
		sendEphemeral(s, i, "エラー：ロールの付与に失敗しました。")
		return
	}

	sendEphemeral(s, i, fmt.Sprintf("%sの%sロールを付与しました。", userPref, targetRoleName))
}

func sendEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
