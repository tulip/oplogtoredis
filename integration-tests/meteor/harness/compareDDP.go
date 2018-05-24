package harness

import (
	"sort"

	"github.com/kylelemons/godebug/pretty"
)

// This is a support utility for DDPConn.VerifyReceive to compare a set
// of expected message groups against a set of received messages, with the
// order within each group not mattering.
func compareDDP(actualMessages DDPMsgGroup, expectedMessageGroups []DDPMsgGroup) string {
	actualMessageGroups := compareDDPPartitionMessages(actualMessages, compareDDPPartitionLengths(expectedMessageGroups))

	compareDDPSortMessageGroups(actualMessageGroups)
	compareDDPSortMessageGroups(expectedMessageGroups)

	return pretty.Compare(actualMessageGroups, expectedMessageGroups)
}

func compareDDPPartitionMessages(messages DDPMsgGroup, partitionLength []int) []DDPMsgGroup {
	paritionedMessages := make([]DDPMsgGroup, len(partitionLength))

	startIdx := 0
	for i, length := range partitionLength {
		endIdx := startIdx + length

		if (startIdx < len(messages)) && (endIdx <= len(messages)) {
			paritionedMessages[i] = messages[startIdx:endIdx]
		} else {
			paritionedMessages[i] = DDPMsgGroup{}
		}

		startIdx = endIdx
	}

	// Create a final partition for any messages that didn't fit
	if len(messages) > startIdx {
		paritionedMessages = append(paritionedMessages, messages[startIdx:])
	}

	return paritionedMessages
}

func compareDDPPartitionLengths(messageGroups []DDPMsgGroup) []int {
	lengths := make([]int, len(messageGroups))
	for i, group := range messageGroups {
		lengths[i] = len(group)
	}

	return lengths
}

func compareDDPSortMessageGroups(messageGroups []DDPMsgGroup) {
	for _, messageGroup := range messageGroups {
		sort.Slice(messageGroup, func(i, j int) bool {
			a := messageGroup[i]
			b := messageGroup[j]

			if a.DDPType != b.DDPType {
				return a.DDPType < b.DDPType
			}

			// Pretty much all DDP messages have an "id" field, and the few
			// that don't shouldn't come up in the same message group
			return a.Data["id"].(string) < b.Data["id"].(string)
		})
	}
}
