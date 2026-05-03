import { apiClient } from './client'
import type {
  GenerateImageRequest,
  GenerateImageResponse,
  ImageCapableKeysResponse,
  ImageGenerationRecord,
  CreateImageGenerationRecordRequest,
  UpdateImageGenerationRecordRequest
} from '@/types'

export async function listImageCapableKeys(): Promise<ImageCapableKeysResponse> {
  const { data } = await apiClient.get<ImageCapableKeysResponse>('/user/image-capable-keys')
  return data
}

function joinImagesEndpoint(apiBaseUrl?: string): string {
  const base = (apiBaseUrl || '').trim().replace(/\/+$/, '')
  if (!base) return '/v1/images/generations'
  if (base.endsWith('/v1')) return `${base}/images/generations`
  return `${base}/v1/images/generations`
}

export async function generateImage(
  apiKey: string,
  request: GenerateImageRequest,
  apiBaseUrl?: string
): Promise<GenerateImageResponse> {
  const response = await fetch(joinImagesEndpoint(apiBaseUrl), {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(request)
  })

  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    const message =
      payload?.error?.message ||
      payload?.message ||
      `Image request failed with HTTP ${response.status}`
    throw new Error(message)
  }

  return payload as GenerateImageResponse
}

export async function listImageGenerationRecords(limit = 50): Promise<ImageGenerationRecord[]> {
  const { data } = await apiClient.get<ImageGenerationRecord[]>('/user/image-generations', {
    params: { limit }
  })
  return data || []
}

export async function createImageGenerationRecord(
  request: CreateImageGenerationRecordRequest
): Promise<ImageGenerationRecord> {
  const { data } = await apiClient.post<ImageGenerationRecord>('/user/image-generations', request)
  return data
}

export async function updateImageGenerationRecord(
  id: number,
  request: UpdateImageGenerationRecordRequest
): Promise<ImageGenerationRecord> {
  const { data } = await apiClient.patch<ImageGenerationRecord>(`/user/image-generations/${id}`, request)
  return data
}

export const imagesAPI = {
  listImageCapableKeys,
  generateImage,
  listImageGenerationRecords,
  createImageGenerationRecord,
  updateImageGenerationRecord
}

export default imagesAPI
