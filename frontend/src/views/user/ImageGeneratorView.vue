<template>
  <AppLayout>
    <div class="mx-auto flex h-[calc(100vh-88px)] max-w-6xl flex-col px-4 py-5 sm:px-6 lg:px-8">
      <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold tracking-normal text-gray-950 dark:text-white">
            {{ t('imageGenerator.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">
            {{ t('imageGenerator.description') }}
          </p>
        </div>
        <button class="btn btn-secondary self-start sm:self-auto" :disabled="loadingKeys" @click="loadWorkspace">
          <Icon name="refresh" size="md" :class="loadingKeys ? 'animate-spin' : ''" />
          <span class="ml-2">{{ t('common.refresh') }}</span>
        </button>
      </div>

      <section class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div
          ref="conversationRef"
          class="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 py-5 sm:px-6"
        >
          <div
            v-if="loadingKeys"
            class="mx-auto flex h-full max-w-xl flex-col justify-center space-y-3"
          >
            <div class="h-16 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-700" />
            <div class="h-28 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-700" />
            <div class="h-16 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-700" />
          </div>

          <div
            v-else-if="capableKeys.length === 0"
            class="flex h-full items-center justify-center px-3 text-center"
          >
            <div class="max-w-md rounded-2xl border border-dashed border-gray-300 bg-gray-50 p-6 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-900/40 dark:text-dark-300">
              {{ emptyReasonText }}
            </div>
          </div>

          <div
            v-else-if="messages.length === 0"
            class="flex h-full items-center justify-center px-3 text-center"
          >
            <div class="max-w-md">
              <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
                <Icon name="sparkles" size="xl" />
              </div>
              <p class="mt-4 text-sm font-medium text-gray-700 dark:text-dark-100">
                {{ t('imageGenerator.emptyResult') }}
              </p>
            </div>
          </div>

          <article
            v-for="message in messages"
            :key="message.id"
            class="flex"
            :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
          >
            <div
              class="max-w-[min(760px,92%)]"
              :class="message.role === 'user' ? 'items-end' : 'items-start'"
            >
              <div
                class="rounded-2xl px-4 py-3 text-sm leading-6"
                :class="message.role === 'user'
                  ? 'bg-primary-600 text-white shadow-sm'
                  : 'border border-gray-200 bg-gray-50 text-gray-800 dark:border-dark-700 dark:bg-dark-900/50 dark:text-dark-100'"
              >
                <p class="whitespace-pre-wrap break-words">{{ message.content }}</p>
                <div
                  v-if="message.meta"
                  class="mt-2 flex flex-wrap gap-2 text-xs"
                  :class="message.role === 'user' ? 'text-primary-100' : 'text-gray-500 dark:text-dark-300'"
                >
                  <span>{{ message.meta.keyName }}</span>
                  <span>{{ message.meta.model }}</span>
                  <span>{{ formatSizeLabel(message.meta.size) }}</span>
                </div>
              </div>

              <div
                v-if="message.loading"
                class="mt-3 flex items-center gap-2 rounded-2xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900/50 dark:text-dark-300"
              >
                <Icon name="refresh" size="sm" class="animate-spin" />
                <span>{{ t('imageGenerator.waiting') }}</span>
              </div>

              <div
                v-if="message.error"
                class="mt-3 rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
              >
                {{ message.error }}
              </div>

              <div v-if="message.images?.length" class="mt-3 grid gap-3 sm:grid-cols-2">
                <article
                  v-for="(image, index) in message.images"
                  :key="`${message.id}-${index}`"
                  class="overflow-hidden rounded-2xl border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900/50"
                >
                  <div class="aspect-square bg-gray-100 dark:bg-dark-900">
                    <img :src="image.src" :alt="t('imageGenerator.imageAlt', { index: index + 1 })" class="h-full w-full object-contain" />
                  </div>
                  <div class="flex items-center justify-between gap-3 p-3">
                    <p class="min-w-0 truncate text-xs text-gray-500 dark:text-dark-300">
                      {{ image.revisedPrompt || message.content }}
                    </p>
                    <button type="button" class="btn btn-secondary shrink-0" @click="downloadImage(image, index)">
                      <Icon name="download" size="sm" />
                      <span class="ml-1.5">{{ t('imageGenerator.download') }}</span>
                    </button>
                  </div>
                </article>
              </div>
            </div>
          </article>
        </div>

        <div class="border-t border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/60 sm:p-4">
          <form class="rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-800" @submit.prevent="submit">
            <textarea
              v-model="prompt"
              :placeholder="t('imageGenerator.promptPlaceholder')"
              :disabled="generating || capableKeys.length === 0"
              rows="3"
              class="min-h-[88px] w-full resize-none border-0 bg-transparent px-1 py-1 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 disabled:cursor-not-allowed disabled:opacity-60 dark:text-white dark:placeholder:text-dark-400"
              @keydown.enter.exact.prevent="submit"
            />

            <div class="mt-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <p class="min-h-5 text-xs text-gray-500 dark:text-dark-300">
                {{ hasPendingGeneration ? t('imageGenerator.pendingTask') : selectedKeyMeta }}
              </p>

              <div class="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(180px,240px)_minmax(150px,200px)_minmax(112px,140px)_44px]">
                <Select
                  v-model="selectedKeyId"
                  :options="keyOptions"
                  searchable
                  :disabled="generating || hasPendingGeneration || capableKeys.length === 0"
                  :placeholder="t('imageGenerator.selectKey')"
                />
                <Select
                  v-model="selectedModel"
                  :options="modelOptions"
                  :disabled="generating || hasPendingGeneration || modelOptions.length === 0"
                  :placeholder="t('imageGenerator.selectModel')"
                />
                <Select
                  v-model="selectedSize"
                  :options="sizeOptions"
                  :disabled="generating || hasPendingGeneration || sizeOptions.length === 0"
                />
                <button
                  type="submit"
                  class="flex h-11 w-full items-center justify-center rounded-xl bg-primary-600 text-white transition hover:bg-primary-700 disabled:cursor-not-allowed disabled:bg-gray-300 disabled:text-gray-500 dark:disabled:bg-dark-600 dark:disabled:text-dark-300"
                  :disabled="generating || hasPendingGeneration || !canSubmit"
                  :aria-label="t('imageGenerator.generate')"
                  :title="hasPendingGeneration ? t('imageGenerator.pendingTask') : t('imageGenerator.generate')"
                >
                  <Icon v-if="generating" name="refresh" size="md" class="animate-spin" />
                  <Icon v-else name="sparkles" size="md" :stroke-width="2" />
                </button>
              </div>
            </div>
          </form>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import { imagesAPI } from '@/api'
import { useAppStore, useImageGenerationStore } from '@/stores'
import type { ImageCapableKey, ImageGenerationStoredImage } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const imageGenerationStore = useImageGenerationStore()
const { messages, generating } = storeToRefs(imageGenerationStore)

const loadingKeys = ref(false)
const capableKeys = ref<ImageCapableKey[]>([])
const defaultModel = ref('gpt-image-2')
const modelCatalog = ref<Record<string, string[]>>({})
const emptyReason = ref<string | null>(null)
const selectedKeyId = ref<number | null>(null)
const selectedModel = ref<string>('')
const selectedSize = ref<string>('1024x1024')
const prompt = ref('')
const conversationRef = ref<HTMLElement | null>(null)

const selectedKey = computed(() => capableKeys.value.find((key) => key.id === selectedKeyId.value) || null)

const keyOptions = computed(() =>
  capableKeys.value.map((key) => ({
    value: key.id,
    label: `${key.name} · ${key.group.name}`,
  }))
)

const modelOptions = computed(() =>
  (selectedKey.value?.models || []).map((model) => ({
    value: model.id,
    label: model.mapped_model && model.mapped_model !== model.id
      ? `${model.id} -> ${model.mapped_model}`
      : model.id,
  }))
)

const sizeOptions = computed(() => {
  const sizes = modelCatalog.value[selectedModel.value] || ['1024x1024']
  const seenLabels = new Set<string>()
  return sizes
    .map((size) => ({
      value: size,
      label: formatSizeLabel(size),
    }))
    .filter((option) => {
      if (option.label !== '1K' && option.label !== '2K') return false
      if (seenLabels.has(option.label)) return false
      seenLabels.add(option.label)
      return true
    })
})

const selectedKeyMeta = computed(() => {
  if (!selectedKey.value) return t('imageGenerator.noKeySelected')
  return `${selectedKey.value.masked_key} · ${selectedKey.value.group.name}`
})

const hasPendingGeneration = computed(() => messages.value.some((message) => message.loading))

const canSubmit = computed(() =>
  !hasPendingGeneration.value &&
  !!selectedKey.value &&
  !!selectedModel.value &&
  !!selectedSize.value &&
  !!prompt.value.trim()
)

const emptyReasonText = computed(() => {
  if (emptyReason.value === 'no_image_capable_key') {
    return t('imageGenerator.noCapableKeys')
  }
  return t('imageGenerator.noCapableKeys')
})

watch(selectedKeyId, () => {
  const preferred = selectedKey.value?.default_model || defaultModel.value
  const candidate = selectedKey.value?.models.find((model) => model.id === preferred)
  selectedModel.value = candidate?.id || selectedKey.value?.models[0]?.id || ''
})

watch(selectedModel, () => {
  const sizes = modelCatalog.value[selectedModel.value] || ['1024x1024']
  if (!sizes.includes(selectedSize.value)) {
    selectedSize.value = sizes[0] || '1024x1024'
  }
})

onMounted(async () => {
  await Promise.all([
    loadWorkspace(),
    imageGenerationStore.loadHistory()
  ])
})

async function loadWorkspace() {
  loadingKeys.value = true
  try {
    const capability = await imagesAPI.listImageCapableKeys()
    capableKeys.value = capability.keys || []
    defaultModel.value = capability.default_model || 'gpt-image-2'
    emptyReason.value = capability.empty_reason
    modelCatalog.value = Object.fromEntries((capability.models || []).map((model) => [model.id, model.sizes || []]))

    if (!selectedKeyId.value || !capableKeys.value.some((key) => key.id === selectedKeyId.value)) {
      selectedKeyId.value = capableKeys.value[0]?.id || null
    }
  } catch (error) {
    appStore.showError((error as Error).message || t('imageGenerator.loadFailed'))
  } finally {
    loadingKeys.value = false
  }
}

async function submit() {
  if (!canSubmit.value || !selectedKey.value || generating.value) return

  const requestPrompt = prompt.value.trim()
  prompt.value = ''

  try {
    await imageGenerationStore.submit({
      apiKeyId: selectedKey.value.id,
      keyName: selectedKey.value.name,
      model: selectedModel.value,
      size: selectedSize.value,
      prompt: requestPrompt,
      labels: {
        generating: t('imageGenerator.generating'),
        generated: t('imageGenerator.generatedMessage'),
        failed: t('imageGenerator.generateFailed'),
        noImageReturned: t('imageGenerator.noImageReturned'),
      },
    })
    await scrollToBottom()
  } catch (error) {
    prompt.value = requestPrompt
    appStore.showError((error as Error).message || t('imageGenerator.generateFailed'))
  }
}

async function downloadImage(image: ImageGenerationStoredImage, index: number) {
  const response = await fetch(image.src)
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `api2code-image-${index + 1}.png`
  anchor.click()
  URL.revokeObjectURL(url)
}

function formatSizeLabel(size: string): string {
  const normalized = size.trim()
  const upper = normalized.toUpperCase()
  if (upper === '1K' || upper === '2K') return upper

  const labels: Record<string, string> = {
    '1024x1024': '1K',
    '1536x1024': '2K',
    '1024x1536': '2K',
    '1792x1024': '2K',
    '1024x1792': '2K',
  }
  return labels[normalized] || normalized
}

async function scrollToBottom() {
  await nextTick()
  const el = conversationRef.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}

</script>
