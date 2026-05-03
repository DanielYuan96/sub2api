import { defineStore } from 'pinia'
import { ref } from 'vue'
import { imagesAPI } from '@/api'
import type { ImageGenerationRecord, ImageGenerationStoredImage } from '@/types'

export type ImageChatMessage = {
  id: string
  role: 'user' | 'assistant'
  content: string
  meta?: {
    keyName: string
    model: string
    size: string
  }
  loading?: boolean
  error?: string
  images?: ImageGenerationStoredImage[]
  recordId?: number
}

export type SubmitImageGenerationInput = {
  apiKeyId: number
  keyName: string
  model: string
  size: string
  prompt: string
  labels: {
    generating: string
    generated: string
    failed: string
    noImageReturned: string
  }
}

export const useImageGenerationStore = defineStore('imageGeneration', () => {
  const messages = ref<ImageChatMessage[]>([])
  const generating = ref(false)
  const historyLoaded = ref(false)
  let pollTimer: number | null = null

  async function loadHistory(force = false) {
    if (historyLoaded.value && !force) return

    const records = await imagesAPI.listImageGenerationRecords(50)
    syncMessagesFromRecords(records)
    historyLoaded.value = true
    updatePolling()
  }

  function syncMessagesFromRecords(records: ImageGenerationRecord[]) {
    messages.value = records
      .slice()
      .reverse()
      .flatMap(recordToMessages)
  }

  async function submit(input: SubmitImageGenerationInput) {
    if (generating.value) return

    const requestPrompt = input.prompt.trim()
    if (!requestPrompt) return

    generating.value = true
    let record: ImageGenerationRecord | null = null
    let assistantId = ''

    try {
      record = await imagesAPI.createImageGenerationRecord({
        api_key_id: input.apiKeyId,
        model: input.model,
        size: input.size,
        prompt: requestPrompt
      })
      assistantId = pushProcessingMessages(record, input.labels.generating)

      startPolling()
    } catch (error) {
      const message = (error as Error).message || input.labels.failed
      if (record && assistantId) {
        updateAssistantMessage(assistantId, {
          content: input.labels.failed,
          loading: false,
          error: message,
        })
      }
      throw error
    } finally {
      generating.value = false
    }
  }

  function pushProcessingMessages(record: ImageGenerationRecord, generatingText: string): string {
    const userId = `image-record-${record.id}-user`
    const assistantId = `image-record-${record.id}-assistant`
    messages.value.push({
      id: userId,
      role: 'user',
      content: record.prompt,
      meta: {
        keyName: record.api_key_name,
        model: record.model,
        size: record.size,
      },
      recordId: record.id,
    })
    messages.value.push({
      id: assistantId,
      role: 'assistant',
      content: generatingText,
      loading: true,
      recordId: record.id,
    })
    return assistantId
  }

  function startPolling() {
    if (pollTimer !== null) return
    pollTimer = window.setInterval(async () => {
      try {
        const records = await imagesAPI.listImageGenerationRecords(50)
        syncMessagesFromRecords(records)
        if (!records.some((record) => record.status === 'processing')) {
          stopPolling()
        }
      } catch {
        // Keep polling; transient auth/network errors should not wipe the chat state.
      }
    }, 2000)
  }

  function stopPolling() {
    if (pollTimer === null) return
    window.clearInterval(pollTimer)
    pollTimer = null
  }

  function updatePolling() {
    if (messages.value.some((message) => message.loading)) {
      startPolling()
    } else {
      stopPolling()
    }
  }

  function updateAssistantMessage(id: string, patch: Partial<ImageChatMessage>) {
    const target = messages.value.find((message) => message.id === id)
    if (!target) return
    Object.assign(target, patch)
  }

  function recordToMessages(record: ImageGenerationRecord): ImageChatMessage[] {
    return [
      {
        id: `image-record-${record.id}-user`,
        role: 'user',
        content: record.prompt,
        meta: {
          keyName: record.api_key_name,
          model: record.model,
          size: record.size,
        },
        recordId: record.id,
      },
      recordToAssistantMessage(record)
    ]
  }

  function recordToAssistantMessage(
    record: ImageGenerationRecord,
    labels?: { generated: string; failed: string; generating: string }
  ): ImageChatMessage {
    const loading = record.status === 'processing'
    const failed = record.status === 'failed'
    const images = record.images || []
    return {
      id: `image-record-${record.id}-assistant`,
      role: 'assistant',
      content: loading
        ? (labels?.generating || '生成中')
        : failed
          ? (labels?.failed || '生成失败')
          : (labels?.generated || '图片已生成'),
      loading,
      error: failed ? record.error_message : undefined,
      images,
      meta: {
        keyName: record.api_key_name,
        model: record.model,
        size: record.size,
      },
      recordId: record.id,
    }
  }

  return {
    messages,
    generating,
    historyLoaded,
    loadHistory,
    submit,
  }
})
