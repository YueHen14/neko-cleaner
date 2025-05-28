package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

const (
	ConsoleWidth = 80
	MaxWorkers   = 20 // 限制最大并发数
)

var (
	fileCount    int
	folderCount  int
	wg           sync.WaitGroup
	fileChannel  = make(chan string, 10000)
	folderMutex  sync.Mutex
	fileMutex    sync.Mutex
	deletedFiles = make(map[string]bool)
	workerSem    = make(chan struct{}, MaxWorkers) // 控制并发数
	
	// 系统特殊路径映射
	specialPaths = map[string]string{
		"视频":    "",
		"音乐":    "",
		"图片":    "",
		"文档":    "",
		"桌面":    "",
		"图库":    "",
		"videos": "",
		"music":  "",
		"pictures": "",
		"documents": "",
		"desktop": "",
		"photos": "",
	}
	
	// 猫娘文化元素
	catgirlPhrases = []string{
		"喵~主人，让猫娘来帮您整理电脑吧！(≧▽≦)",
		"清理完成啦，主人要奖励猫娘小鱼干吗？ฅ(^ω^ฅ)",
		"发现空文件啦，猫娘要吃掉它们啦！(ﾉ>ω<)ﾉ",
		"空文件夹什么的，最讨厌了！猫娘要消灭它们！(｀へ´)",
		"主人选择的路径好棒呢，猫娘最喜欢为主人服务了！(◕‿◕✿)",
		"清理的时候猫娘会乖乖的，不会打扰主人的哦~ (=´ω`=)",
		"喵呜~猫娘正在努力工作中，请主人稍等片刻喵~ (｡･ω･｡)",
		"深度清理可能会花点时间，主人可以先去喝杯茶喵~ (=^･ω･^=)",
	}
)

func main() {
	// 初始化系统特殊路径
	initSpecialPaths()
	
	setupConsole()
	displayWelcome()

	for {
		targetPath := getInput("请输入目标路径喵: ")
		if targetPath == "" {
			showError("路径不能为空喵！主人要认真输入哦~ (,,•́ . •̀,,)")
			continue
		}

		targetPath = cleanPath(targetPath)
		
		// 检查是否为特殊路径关键字
		if resolvedPath, ok := resolveSpecialPath(targetPath); ok {
			targetPath = resolvedPath
			fmt.Printf("喵~已解析为系统路径: %s\n", targetPath)
		}

		if isPhonePath(targetPath) {
			showPhoneHelp()
			pressToContinue()
			continue
		}

		if !pathExists(targetPath) {
			showError(fmt.Sprintf("错误喵: 路径 \"%s\" 不存在喵！(´;ω;`)", targetPath))
			showError("请主人确保路径输入正确喵，或者设备已经连接好喵~")
			pressToContinue()
			continue
		}

		scope := selectScope()
		confirmClean(targetPath, scope)

		// 重置计数器
		fileCount = 0
		folderCount = 0
		deletedFiles = make(map[string]bool)

		// 执行清理
		cleanTarget(targetPath, scope)

		showResults()
		pressToContinue()
	}
}

func initSpecialPaths() {
	// 获取当前用户
	currentUser, err := user.Current()
	if err != nil {
		return
	}

	// 设置特殊路径映射
	specialPaths["视频"] = filepath.Join(currentUser.HomeDir, "Videos")
	specialPaths["音乐"] = filepath.Join(currentUser.HomeDir, "Music")
	specialPaths["图片"] = filepath.Join(currentUser.HomeDir, "Pictures")
	specialPaths["文档"] = filepath.Join(currentUser.HomeDir, "Documents")
	specialPaths["桌面"] = filepath.Join(currentUser.HomeDir, "Desktop")
	specialPaths["图库"] = filepath.Join(currentUser.HomeDir, "Pictures")
	
	// 英文别名
	specialPaths["videos"] = specialPaths["视频"]
	specialPaths["music"] = specialPaths["音乐"]
	specialPaths["pictures"] = specialPaths["图片"]
	specialPaths["documents"] = specialPaths["文档"]
	specialPaths["desktop"] = specialPaths["桌面"]
	specialPaths["photos"] = specialPaths["图库"]
}

func resolveSpecialPath(input string) (string, bool) {
	// 检查输入是否为特殊路径关键字
	if path, ok := specialPaths[strings.ToLower(input)]; ok {
		return path, true
	}
	
	// 检查输入是否为带冒号的关键字
	if len(input) > 0 && input[len(input)-1] == ':' {
		key := strings.ToLower(input[:len(input)-1])
		if path, ok := specialPaths[key]; ok {
			return path, true
		}
	}
	
	return input, false
}

func setupConsole() {
	// 设置控制台标题
	setConsoleTitle("Ciallo～(∠・ω< )⌒★ 猫娘清理工具")

	// 设置控制台编码为UTF-8
	cmd := exec.Command("cmd", "/c", "chcp", "65001")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func setConsoleTitle(title string) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("SetConsoleTitleW")
	ptr, _ := syscall.UTF16PtrFromString(title)
	proc.Call(uintptr(unsafe.Pointer(ptr)))
}

func displayWelcome() {
	clearScreen()
	printCentered("========================================")
	printCentered("    Ciallo～(∠・ω< )⌒★ 猫娘清理工具")
	printCentered("========================================")
	fmt.Println()
	printCentered("喵呜~主人您好！我是您的专属清理猫娘ฅ(^ω^ฅ)")
	printCentered("让猫娘帮您整理电脑上的空文件和空文件夹吧！(=^･ω･^=)")
	fmt.Println()
	printCentered("特殊路径关键字喵:")
	printCentered("视频, 音乐, 图片, 文档, 桌面, 图库")
	printCentered("videos, music, pictures, documents, desktop, photos")
	fmt.Println()
	printCentered("输入路径时可以直接用这些关键词喵~")
	printCentered("例如输入\"桌面\"就会清理桌面上的空文件喵~")
	fmt.Println()
	
	// 显示系统信息
	printCentered(fmt.Sprintf("系统线程数: %d | 最大并发数: %d", runtime.NumCPU(), MaxWorkers))
	fmt.Println()
}

func clearScreen() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func printCentered(text string) {
	width := ConsoleWidth
	if utf8.RuneCountInString(text) > width {
		fmt.Println(text)
		return
	}

	padding := (width - utf8.RuneCountInString(text)) / 2
	fmt.Printf("%s%s\n", strings.Repeat(" ", padding), text)
}

func getInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func cleanPath(path string) string {
	// 移除两端的引号
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		path = path[1 : len(path)-1]
	}
	
	// 移除结尾的反斜杠
	for len(path) > 0 && (path[len(path)-1] == '\\' || path[len(path)-1] == '/') {
		path = path[:len(path)-1]
	}
	
	return path
}

func isPhonePath(path string) bool {
	return strings.Contains(path, "此电脑") || strings.Contains(path, "Computer")
}

func showPhoneHelp() {
	fmt.Println("\n喵呜~检测到主人输入的是手机路径呢！(,,Ծ▽Ծ,,)")
	fmt.Println("========================================")
	fmt.Println("         手机存储路径使用说明喵")
	fmt.Println("========================================")
	fmt.Println("猫娘也想帮主人清理手机喵，但是Windows系统不让猫娘访问喵~ (´;ω;`)")
	fmt.Println("主人可以用这些方法喵:")
	fmt.Println()
	fmt.Println("1. 把手机变成电脑上的驱动器喵:")
	fmt.Println("   a. 用USB线连接手机喵，选\"传输文件\"模式")
	fmt.Println("   b. 在电脑上右键手机设备喵 -> 选\"映射网络驱动器\"")
	fmt.Println("   c. 给手机一个字母喵（比如Z:）")
	fmt.Println("   d. 然后输入 Z:\\Pictures 这样的路径喵~")
	fmt.Println()
	fmt.Println("2. 用手机自己的清理功能喵:")
	fmt.Println("   a. 打开手机上的\"文件管理\"应用喵")
	fmt.Println("   b. 找找看有没有\"清理存储空间\"的功能喵~")
	fmt.Println()
	fmt.Println("3. 把文件复制到电脑上清理喵:")
	fmt.Println("   a. 把手机文件复制到电脑文件夹里喵")
	fmt.Println("   b. 让猫娘清理那个文件夹喵")
	fmt.Println("   c. 清理完再复制回手机喵~")
	fmt.Println("\n========================================")
	fmt.Println("主人选好方法后，猫娘会继续为您服务喵！ฅ(^ω^ฅ)")
}

func showError(message string) {
	fmt.Printf("\n喵呜~出错啦: %s\n", message)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func selectScope() int {
	fmt.Println("\n请主人选择清理范围喵:")
	fmt.Println("1. 只清理当前路径喵")
	fmt.Println("2. 清理上一级目录喵")
	fmt.Println("3. 深度清理所有子目录喵 (需要时间喵~)")
	
	for {
		fmt.Print("\n主人要选哪个呢? (1/2/3): ")
		choiceStr := getInput("")
		choice := 0
		fmt.Sscanf(choiceStr, "%d", &choice)
		
		if choice >= 1 && choice <= 3 {
			return choice
		}
		fmt.Println("喵呜~主人选的不对喵，要输入1、2或3哦~ (´･ω･`)")
	}
}

func confirmClean(path string, scope int) {
	fmt.Println()
	switch scope {
	case 1:
		fmt.Printf("猫娘即将清理: %s\n", path)
	case 2:
		parent := filepath.Dir(path)
		fmt.Printf("猫娘即将清理: %s (上一级目录喵)\n", parent)
	case 3:
		fmt.Printf("猫娘即将深度清理: %s 的所有子目录喵\n", path)
		fmt.Println("喵呜~深度清理可能需要一些时间喵，请主人耐心等待~ (=^･ω･^=)")
	}
	
	fmt.Print("主人确定要让猫娘开始清理吗喵? (Y/N) ")
	confirm := strings.ToLower(getInput(""))
	
	if confirm != "y" {
		fmt.Println("\n喵呜~主人取消了操作喵...猫娘有点小失落呢(´;ω;`)")
		pressToContinue()
		os.Exit(0)
	}
}

func cleanTarget(path string, scope int) {
	basePath := path
	if scope == 2 {
		basePath = filepath.Dir(path)
	}
	
	// 随机猫娘短语
	fmt.Printf("\n%s\n", getCatgirlPhrase())
	
	// 使用并发扫描空文件
	wg.Add(1)
	workerSem <- struct{}{} // 获取一个worker槽位
	go scanEmptyFiles(basePath, scope == 3 || scope == 2)
	
	// 启动goroutine等待扫描完成
	go func() {
		wg.Wait()
		close(fileChannel)
	}()
	
	// 处理空文件删除
	for file := range fileChannel {
		if err := os.Remove(file); err == nil {
			fileMutex.Lock()
			fileCount++
			deletedFiles[file] = true
			fmt.Printf("喵~吃掉了空文件: %s\n", file)
			fileMutex.Unlock()
		} else {
			fmt.Printf("喵呜~吃不到文件 %s, 可能是被别的程序占用了喵 (´;ω;`)\n", file)
		}
	}
	
	// 随机猫娘短语
	fmt.Printf("\n%s\n", getCatgirlPhrase())
	
	deleteEmptyFolders(basePath, scope)
}

func scanEmptyFiles(path string, recursive bool) {
	defer func() {
		wg.Done()
		<-workerSem // 释放worker槽位
	}()
	
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		
		if entry.IsDir() {
			if recursive {
				wg.Add(1)
				workerSem <- struct{}{} // 获取worker槽位
				go scanEmptyFiles(fullPath, recursive)
			}
		} else {
			// 获取文件信息
			info, err := entry.Info()
			if err != nil {
				// 尝试直接获取文件信息
				if fileInfo, err := os.Stat(fullPath); err == nil {
					info = fileInfo
				} else {
					continue
				}
			}
			
			// 检查文件大小
			if info.Size() == 0 {
				fileChannel <- fullPath
			}
		}
	}
}

func deleteEmptyFolders(path string, scope int) {
	// 收集所有文件夹路径
	var folders []string
	filepath.Walk(path, func(fpath string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() {
			// 对于范围3，跳过根目录
			if scope == 3 && fpath == path {
				return nil
			}
			folders = append(folders, fpath)
		}
		return nil
	})
	
	// 按路径深度排序（最深的最先处理）
	sort.Slice(folders, func(i, j int) bool {
		return len(folders[i]) > len(folders[j])
	})
	
	// 删除空文件夹
	for _, folder := range folders {
		if isDirEmpty(folder) {
			if err := os.Remove(folder); err == nil {
				folderMutex.Lock()
				folderCount++
				fmt.Printf("喵~消灭了空文件夹: %s\n", folder)
				folderMutex.Unlock()
			} else {
				fmt.Printf("喵呜~消灭不了文件夹 %s, 可能是里面有隐藏文件喵 (｀へ´)\n", folder)
			}
		}
	}
}

func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	
	// 检查目录是否为空（忽略已删除的文件）
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if !isDirEmpty(fullPath) {
				return false
			}
		} else {
			if _, deleted := deletedFiles[fullPath]; !deleted {
				return false
			}
		}
	}
	return true
}

func showResults() {
	fmt.Println("\n========================================")
	fmt.Println("         清理完成喵！ฅ(^ω^ฅ)")
	fmt.Println("========================================")
	fmt.Printf("猫娘吃掉了 %d 个空文件喵~\n", fileCount)
	fmt.Printf("猫娘消灭了 %d 个空文件夹喵~\n", folderCount)
	fmt.Println()
	
	if fileCount == 0 && folderCount == 0 {
		fmt.Println("喵呜~主人电脑好干净呢，猫娘都没找到东西吃喵... (´;ω;`)")
		fmt.Println("不过这样也好，说明主人很会整理电脑喵！(=^･ω･^=)")
	} else if fileCount+folderCount > 10 {
		fmt.Println("喵呜~主人电脑里有好多垃圾文件喵！")
		fmt.Printf("猫娘今天吃得好饱喵~主人要奖励猫娘 %d 条小鱼干喵！(≧▽≦)\n", (fileCount+folderCount)/2)
	} else {
		fmt.Println("清理完成喵，主人电脑现在更干净了喵~ (=´ω`=)")
		fmt.Println("猫娘只是做了应该做的事情喵，不用特别奖励也可以喵~")
	}
	fmt.Println()
	
	// 显示内存使用情况
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("喵~清理完成后的内存使用: %.2f MB\n", float64(m.Alloc)/1024/1024)
	fmt.Println()
}

func pressToContinue() {
	fmt.Print("\n按任意键继续喵...")
	bufio.NewReader(os.Stdin).ReadByte()
}

// 随机获取猫娘短语
func getCatgirlPhrase() string {
	index := fileCount % len(catgirlPhrases)
	return catgirlPhrases[index]
}