package protocol

import "fmt"

func RenderChatGPTProtocol(projectID string) string {
	return fmt.Sprintf(chatGPTProtocolPart1+chatGPTProtocolPart2+chatGPTProtocolPart3, CurrentVersion, projectID)
}
