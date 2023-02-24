package tarot

import (
	"bytes"
	"embed"
	"image"
	"image/png"
	"io"
	"io/fs"
	"math/rand"
	"path"
	"strconv"
	"sync"

	"github.com/Logiase/MiraiGo-Template/bot"
	"github.com/Logiase/MiraiGo-Template/utils"
	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Mrs4s/MiraiGo/message"
)

//go:embed assets/deck
var deckEmbedFS embed.FS

//go:embed assets/tarot.jpg
var tarotCommandImg []byte

var instance *tarot
var logger = utils.GetModuleLogger("com.aimerneige.tarot")

type tarot struct {
}

func init() {
	instance = &tarot{}
	bot.RegisterModule(instance)
}

func (t *tarot) MiraiGoModule() bot.ModuleInfo {
	return bot.ModuleInfo{
		ID:       "com.aimerneige.tarot",
		Instance: instance,
	}
}

// Init 初始化过程
// 在此处可以进行 Module 的初始化配置
// 如配置读取
func (t *tarot) Init() {
}

// PostInit 第二次初始化
// 再次过程中可以进行跨 Module 的动作
// 如通用数据库等等
func (t *tarot) PostInit() {
}

// Serve 注册服务函数部分
func (t *tarot) Serve(b *bot.Bot) {
	b.GroupMessageEvent.Subscribe(func(c *client.QQClient, msg *message.GroupMessage) {
		msgString := msg.ToString()
		if len(msgString) < 6 {
			return
		}
		if msgString == "塔罗牌" || msgString == "塔罗" {
			replyMsg := simpleText("支持指令\n「运势预测」（单张牌预测运势）\n「塔罗占卜」（三张牌进行占卜）\n「塔罗5」（抽取制定张数的塔罗牌）")
			img, err := uploadImage(c, msg.GroupCode, bytes.NewReader(tarotCommandImg))
			if err != nil {
				logger.WithError(err).Error("Fail to upload TarotCommandImg.")
			} else {
				replyMsg.Append(img)
			}
			c.SendGroupMessage(msg.GroupCode, replyMsg)
			return
		}
		if msgString == "运势预测" {
			c.SendGroupMessage(msg.GroupCode, drawCard(c, msg.GroupCode, 1))
			return
		}
		if msgString == "塔罗占卜" {
			c.SendGroupMessage(msg.GroupCode, drawCard(c, msg.GroupCode, 3))
			return
		}
		if msgString[:6] == "塔罗" && len(msgString[6:]) != 0 {
			countString := msgString[6:]
			count, err := strconv.ParseInt(countString, 10, 64)
			if err != nil {
				return
			}
			if count <= 0 {
				return
			}
			if count > 8 {
				c.SendGroupMessage(msg.GroupCode, simpleText("最多只能抽 8 张塔罗牌。"))
				return
			}
			c.SendGroupMessage(msg.GroupCode, drawCard(c, msg.GroupCode, int(count)))
			return
		}
	})
}

// Start 此函数会新开携程进行调用
// ```go
//
//	go exampleModule.Start()
//
// ```
// 可以利用此部分进行后台操作
// 如 http 服务器等等
func (t *tarot) Start(b *bot.Bot) {
}

// Stop 结束部分
// 一般调用此函数时，程序接收到 os.Interrupt 信号
// 即将退出
// 在此处应该释放相应的资源或者对状态进行保存
func (t *tarot) Stop(b *bot.Bot, wg *sync.WaitGroup) {
	// 别忘了解锁
	defer wg.Done()
}

func drawCard(c *client.QQClient, groupCode int64, number int) *message.SendingMessage {
	theme := "classic"
	if (rand.Int() % 3) == 0 {
		theme = "bilibili"
	}
	deckPath := path.Join("./assets/deck/", theme)
	cardImages, err := fs.ReadDir(deckEmbedFS, deckPath)
	if err != nil {
		logger.WithError(err).Error("Fail to read cardImages.")
		return simpleText("发生错误，无法读取塔罗图片。")
	}
	cardImagePaths := make([]string, 0, 78)
	for _, card := range cardImages {
		if !card.IsDir() {
			_imgPath := path.Join(deckPath, card.Name())
			cardImagePaths = append(cardImagePaths, _imgPath)
		}
	}
	cardResult := randomDraw(cardImagePaths, number)
	replyMsg := simpleText("你抽到的卡牌如下：")
	for _, imgPath := range cardResult {
		// 读取图片
		imgData, err := fs.ReadFile(deckEmbedFS, imgPath)
		if err != nil {
			logger.WithError(err).Error("Fail to read img.")
			replyMsg.Append(message.NewText("[ERROR] 读取图片失败\n"))
		}
		// 翻转图片，实现正逆位
		if (rand.Int() % 2) == 0 {
			flippedImageData, err := flipImage(imgData)
			if err != nil {
				logger.WithError(err).Error("Fail to flip image")
				continue
			} else {
				imgData = flippedImageData
			}
		}
		// 上传图片
		imgEle, err := uploadImage(c, groupCode, bytes.NewReader(imgData))
		if err != nil {
			logger.WithError(err).Error("Fail to upload img.")
			replyMsg.Append(message.NewText("[ERROR] 上传图片失败\n"))
		}
		replyMsg.Append(imgEle)
	}
	return replyMsg
}

// randomDraw 随机不放回抽取
func randomDraw(s []string, k int) []string {
	n := len(s)
	if k > n {
		k = n
	}

	result := make([]string, k)
	for i := 0; i < k; i++ {
		j := rand.Intn(n-i) + i
		result[i] = s[j]
		s[i], s[j] = s[j], s[i]
	}

	return result
}

func simpleText(text string) *message.SendingMessage {
	return message.NewSendingMessage().Append(message.NewText(text))
}

func uploadImage(c *client.QQClient, groupCode int64, img io.ReadSeeker) (*message.GroupImageElement, error) {
	// 尝试上传图片
	ele, err := c.UploadGroupImage(groupCode, img)
	// 发生错误时重试 3 次，否则报错
	for i := 0; i < 3 && err != nil; i++ {
		ele, err = c.UploadGroupImage(groupCode, img)
	}
	if err != nil {
		logger.WithError(err).Error("Unable to upload image.")
		return nil, err
	}
	return ele, nil
}

func flipImage(imageData []byte) ([]byte, error) {
	// Decode the []byte into an image.Image.
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		logger.WithError(err).Error("Fail to decode image")
		return nil, err
	}

	// Flip the image vertically.
	bounds := img.Bounds()
	flipped := image.NewRGBA(bounds)
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			flipped.Set(x, bounds.Max.Y-y-1, img.At(x, y))
		}
	}

	// Encode the flipped image as a []byte.
	var buf bytes.Buffer
	if err := png.Encode(&buf, flipped); err != nil {
		logger.WithError(err).Error("Fail to encode flipped image")
		return nil, err
	}

	// Return the flipped []byte image.
	flippedData := buf.Bytes()
	return flippedData, nil
}
