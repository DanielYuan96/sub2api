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
                  v-if="message.referenceImages?.length"
                  class="mt-3 flex flex-wrap gap-2"
                  :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
                >
                  <button
                    v-for="(reference, index) in message.referenceImages"
                    :key="`${message.id}-ref-${index}`"
                    type="button"
                    class="overflow-hidden rounded-xl border transition hover:brightness-105 focus:outline-none focus:ring-2 focus:ring-primary-400"
                    :class="message.role === 'user' ? 'border-white/25 bg-white/10' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'"
                    @click="openImagePreview(reference.src, t('imageGenerator.referenceImageAlt', { index: index + 1 }))"
                  >
                    <img
                      :src="reference.src"
                      :alt="t('imageGenerator.referenceImageAlt', { index: index + 1 })"
                      class="block h-auto max-h-40 max-w-[min(220px,62vw)] object-contain"
                    />
                  </button>
                </div>
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

              <div v-if="message.images?.length" class="mt-3 flex flex-col items-start gap-3">
                <article
                  v-for="(image, index) in message.images"
                  :key="`${message.id}-${index}`"
                  class="max-w-full"
                >
                  <button
                    type="button"
                    class="block max-w-full overflow-hidden rounded-2xl border border-gray-200 bg-gray-100 transition hover:brightness-105 focus:outline-none focus:ring-2 focus:ring-primary-400 dark:border-dark-700 dark:bg-dark-900"
                    @click="openImagePreview(image.src, t('imageGenerator.imageAlt', { index: index + 1 }))"
                  >
                    <img
                      :src="image.src"
                      :alt="t('imageGenerator.imageAlt', { index: index + 1 })"
                      class="block h-auto max-h-[62vh] max-w-[min(680px,92vw)] object-contain"
                    />
                  </button>
                  <div class="mt-2 flex max-w-[min(680px,92vw)] items-center justify-between gap-3">
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
          <form
            class="rounded-2xl border bg-white p-3 shadow-sm transition dark:bg-dark-800"
            :class="isReferenceDragOver
              ? 'border-primary-400 ring-2 ring-primary-500/20 dark:border-primary-500'
              : 'border-gray-200 dark:border-dark-700'"
            @submit.prevent="submit"
            @dragenter.prevent="handleReferenceDragEnter"
            @dragover.prevent="handleReferenceDragOver"
            @dragleave.prevent="handleReferenceDragLeave"
            @drop.prevent="handleReferenceDrop"
          >
            <textarea
              v-model="prompt"
              :placeholder="t('imageGenerator.promptPlaceholder')"
              :disabled="generating || capableKeys.length === 0"
              rows="3"
              class="min-h-[88px] w-full resize-none border-0 bg-transparent px-1 py-1 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 disabled:cursor-not-allowed disabled:opacity-60 dark:text-white dark:placeholder:text-dark-400"
              @keydown.enter.exact.prevent="submit"
            />

            <div v-if="referenceImages.length" class="mt-3 flex flex-wrap gap-2">
              <article
                v-for="(reference, index) in referenceImages"
                :key="`${reference.name || 'reference'}-${index}`"
                class="group relative overflow-hidden rounded-xl border border-gray-200 bg-gray-100 dark:border-dark-700 dark:bg-dark-900"
              >
                <button
                  type="button"
                  class="block overflow-hidden"
                  @click="openImagePreview(reference.src, t('imageGenerator.referenceImageAlt', { index: index + 1 }))"
                >
                  <img
                    :src="reference.src"
                    :alt="t('imageGenerator.referenceImageAlt', { index: index + 1 })"
                    class="block h-auto max-h-20 max-w-28 object-contain"
                  />
                </button>
                <button
                  type="button"
                  class="absolute right-1 top-1 flex h-6 w-6 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition group-hover:opacity-100"
                  :aria-label="t('imageGenerator.removeReferenceImage')"
                  :title="t('imageGenerator.removeReferenceImage')"
                  @click.stop="removeReferenceImage(index)"
                >
                  <Icon name="x" size="xs" :stroke-width="2" />
                </button>
              </article>
            </div>

            <div class="mt-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <p class="min-h-5 text-xs text-gray-500 dark:text-dark-300">
                {{ hasPendingGeneration ? t('imageGenerator.pendingTask') : selectedKeyMeta }}
              </p>

              <div class="grid grid-cols-1 gap-2 sm:grid-cols-[44px_minmax(180px,240px)_minmax(150px,200px)_44px]">
                <button
                  type="button"
                  class="flex h-11 w-full items-center justify-center rounded-xl border border-gray-200 text-gray-600 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-700"
                  :disabled="!canAddReferenceImages"
                  :aria-label="t('imageGenerator.addReferenceImage')"
                  :title="t('imageGenerator.addReferenceImage')"
                  @click="openReferencePicker"
                >
                  <Icon name="upload" size="md" />
                </button>
                <input
                  ref="referenceInputRef"
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  multiple
                  class="hidden"
                  @change="handleReferenceFileChange"
                />
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
                <button
                  type="button"
                  class="flex h-11 w-full items-center justify-center rounded-xl text-white transition disabled:cursor-not-allowed disabled:bg-gray-300 disabled:text-gray-500 dark:disabled:bg-dark-600 dark:disabled:text-dark-300"
                  :class="hasPendingGeneration
                    ? 'bg-red-500 hover:bg-red-600'
                    : 'bg-primary-600 hover:bg-primary-700'"
                  :disabled="primaryActionDisabled"
                  :aria-label="hasPendingGeneration ? t('imageGenerator.cancelTask') : t('imageGenerator.generate')"
                  :title="hasPendingGeneration ? t('imageGenerator.cancelTask') : t('imageGenerator.generate')"
                  @click="handlePrimaryAction"
                >
                  <Icon v-if="isCancellingPending || generating" name="refresh" size="md" class="animate-spin" />
                  <Icon v-else-if="hasPendingGeneration" name="stop" size="md" :stroke-width="2" />
                  <Icon v-else name="sparkles" size="md" :stroke-width="2" />
                </button>
              </div>
            </div>
          </form>
        </div>
      </section>

      <Teleport to="body">
        <Transition name="fade">
          <div
            v-if="previewImage"
            class="fixed inset-0 z-[100] flex items-center justify-center bg-black/85 p-4 backdrop-blur-sm"
            @click.self="closeImagePreview"
          >
            <button
              type="button"
              class="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-white transition hover:bg-white/20"
              :aria-label="t('common.close')"
              :title="t('common.close')"
              @click="closeImagePreview"
            >
              <Icon name="x" size="lg" :stroke-width="2" />
            </button>
            <img
              :src="previewImage.src"
              :alt="previewImage.alt"
              class="max-h-[90vh] max-w-[94vw] rounded-2xl object-contain shadow-2xl"
            />
          </div>
        </Transition>
      </Teleport>
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
import type { ImageCapableKey, ImageGenerationReferenceImage, ImageGenerationStoredImage } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const imageGenerationStore = useImageGenerationStore()
const { messages, generating } = storeToRefs(imageGenerationStore)

const loadingKeys = ref(false)
const capableKeys = ref<ImageCapableKey[]>([])
const defaultModel = ref('gpt-image-2')
const emptyReason = ref<string | null>(null)
const selectedKeyId = ref<number | null>(null)
const selectedModel = ref<string>('')
const selectedSize = 'auto'
const prompt = ref('')
const conversationRef = ref<HTMLElement | null>(null)
const referenceInputRef = ref<HTMLInputElement | null>(null)
const referenceImages = ref<ImageGenerationReferenceImage[]>([])
const isReferenceDragOver = ref(false)
const referenceDragDepth = ref(0)
const cancellingRecordId = ref<number | null>(null)
const previewImage = ref<{ src: string; alt: string } | null>(null)
const maxReferenceImages = 3
const maxReferenceImageBytes = 5 * 1024 * 1024

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

const selectedKeyMeta = computed(() => {
  if (!selectedKey.value) return t('imageGenerator.noKeySelected')
  return `${selectedKey.value.masked_key} · ${selectedKey.value.group.name}`
})

const hasPendingGeneration = computed(() => messages.value.some((message) => message.loading))
const pendingGenerationRecordId = computed(() =>
  messages.value.find((message) => message.loading && message.recordId)?.recordId || null
)
const isCancellingPending = computed(() =>
  pendingGenerationRecordId.value !== null && cancellingRecordId.value === pendingGenerationRecordId.value
)
const canAddReferenceImages = computed(() =>
  !generating.value &&
  !hasPendingGeneration.value &&
  capableKeys.value.length > 0 &&
  referenceImages.value.length < maxReferenceImages
)

const canSubmit = computed(() =>
  !hasPendingGeneration.value &&
  !!selectedKey.value &&
  !!selectedModel.value &&
  !!prompt.value.trim()
)
const primaryActionDisabled = computed(() => {
  if (hasPendingGeneration.value) {
    return !pendingGenerationRecordId.value || isCancellingPending.value
  }
  return generating.value || !canSubmit.value
})

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
  const requestReferenceImages = referenceImages.value.slice()
  prompt.value = ''
  referenceImages.value = []

  try {
    await imageGenerationStore.submit({
      apiKeyId: selectedKey.value.id,
      keyName: selectedKey.value.name,
      model: selectedModel.value,
      size: selectedSize,
      prompt: requestPrompt,
      referenceImages: requestReferenceImages,
      labels: {
        generating: t('imageGenerator.generating'),
        generated: t('imageGenerator.generatedMessage'),
        failed: t('imageGenerator.generateFailed'),
        cancelled: t('imageGenerator.cancelledMessage'),
        noImageReturned: t('imageGenerator.noImageReturned'),
      },
    })
    await scrollToBottom()
  } catch (error) {
    prompt.value = requestPrompt
    referenceImages.value = requestReferenceImages
    appStore.showError((error as Error).message || t('imageGenerator.generateFailed'))
  }
}

async function handlePrimaryAction() {
  if (hasPendingGeneration.value) {
    if (pendingGenerationRecordId.value) {
      await cancelGeneration(pendingGenerationRecordId.value)
    }
    return
  }
  await submit()
}

async function cancelGeneration(recordId: number) {
  if (cancellingRecordId.value) return
  cancellingRecordId.value = recordId
  try {
    await imageGenerationStore.cancel(recordId, {
      cancelled: t('imageGenerator.cancelledMessage'),
      failed: t('imageGenerator.cancelFailed'),
    })
  } catch (error) {
    appStore.showError((error as Error).message || t('imageGenerator.cancelFailed'))
  } finally {
    cancellingRecordId.value = null
  }
}

function openReferencePicker() {
  referenceInputRef.value?.click()
}

async function handleReferenceFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files || [])
  target.value = ''
  await addReferenceFiles(files)
}

function handleReferenceDragEnter(event: DragEvent) {
  if (!hasDraggedFiles(event)) return
  referenceDragDepth.value += 1
  if (canAddReferenceImages.value) {
    isReferenceDragOver.value = true
  }
}

function handleReferenceDragOver(event: DragEvent) {
  if (!hasDraggedFiles(event)) return
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = canAddReferenceImages.value ? 'copy' : 'none'
  }
}

function handleReferenceDragLeave(event: DragEvent) {
  if (!hasDraggedFiles(event)) return
  referenceDragDepth.value = Math.max(0, referenceDragDepth.value - 1)
  if (referenceDragDepth.value === 0) {
    isReferenceDragOver.value = false
  }
}

async function handleReferenceDrop(event: DragEvent) {
  referenceDragDepth.value = 0
  isReferenceDragOver.value = false
  if (!canAddReferenceImages.value) return
  const files = Array.from(event.dataTransfer?.files || [])
  await addReferenceFiles(files)
}

function hasDraggedFiles(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types || []).includes('Files')
}

async function addReferenceFiles(files: File[]) {
  if (files.length === 0) return

  for (const file of files) {
    if (referenceImages.value.length >= maxReferenceImages) {
      appStore.showError(t('imageGenerator.referenceImageLimit', { count: maxReferenceImages }))
      break
    }
    if (!/^image\/(png|jpeg|webp)$/.test(file.type)) {
      appStore.showError(t('imageGenerator.referenceImageTypeError'))
      continue
    }
    if (file.size > maxReferenceImageBytes) {
      appStore.showError(t('imageGenerator.referenceImageSizeError'))
      continue
    }
    const src = await readFileAsDataURL(file)
    referenceImages.value.push({
      src,
      name: file.name,
      contentType: file.type,
      size: file.size,
    })
  }
}

function removeReferenceImage(index: number) {
  referenceImages.value.splice(index, 1)
}

function openImagePreview(src: string, alt: string) {
  if (!src) return
  previewImage.value = { src, alt }
}

function closeImagePreview() {
  previewImage.value = null
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error || new Error('Failed to read image'))
    reader.readAsDataURL(file)
  })
}

async function downloadImage(image: ImageGenerationStoredImage, index: number) {
  const filename = `api2code-image-${index + 1}.png`
  const src = image.src || ''
  if (!src) return

  try {
    if (src.startsWith('data:')) {
      triggerImageDownload(src, filename)
      return
    }

    const response = await fetch(src, { mode: 'cors' })
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    triggerImageDownload(url, filename)
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch {
    window.open(src, '_blank', 'noopener,noreferrer')
  }
}

function triggerImageDownload(url: string, filename: string) {
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.rel = 'noopener noreferrer'
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

function formatSizeLabel(size: string): string {
  const normalized = size.trim()
  if (normalized.toLowerCase() === 'auto') return t('imageGenerator.autoSize')
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
