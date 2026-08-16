import { useMutation } from '@connectrpc/connect-query'
import { useCallback } from 'react'
import {
  cacheRemoteImage,
  uploadImage,
} from '../gen/purser/domain/v1/image_blob-ImageBlobService_connectquery'
import { createImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
import type { Image } from '../gen/purser/domain/v1/image_pb'

export interface AttachImageTarget {
  ownerType: string
  ownerId: string
  imageType: string
  priority?: number
}

export interface AttachFromUrlParams extends AttachImageTarget {
  url: string
  // source is the provider name to record on the Image row (e.g.
  // "fanart.tv", "theaudiodb") — the caller's choice, since the same
  // hook serves every provider's "pick a lookup result" flow.
  source: string
}

export interface AttachFromFileParams extends AttachImageTarget {
  data: Uint8Array
}

// useAttachImage is the one place the two-call sequence
// docs/technical/image-caching-and-serving.md requires — blob RPC, then
// ImageService.CreateImage — is written, so every consuming screen
// (Person photo, Artist poster, Album cover, ...) reuses it instead of
// re-deriving the call order itself.
//
// Deliberately returns the three underlying connect-query mutation
// objects (cacheRemoteImage/uploadImage/createImage) rather than
// collapsing them into one status: a CacheRemoteImage/UploadImage
// failure leaves nothing orphaned (nothing was ever created), a
// CreateImage failure leaves an orphaned blob per the doc's
// accepted-risk note — the UI needs to tell those apart, so the state
// stays per-call, same one-mutation-object-per-RPC shape useSettings.ts
// already established.
export function useAttachImage() {
  const cacheRemoteImageMutation = useMutation(cacheRemoteImage)
  const uploadImageMutation = useMutation(uploadImage)
  const createImageMutation = useMutation(createImage)

  const attachFromUrl = useCallback(
    async (params: AttachFromUrlParams): Promise<Image> => {
      const { url, source, ownerType, ownerId, imageType, priority } = params
      const cacheResponse = await cacheRemoteImageMutation.mutateAsync({ url })
      const blob = cacheResponse.blob
      if (!blob) {
        throw new Error('CacheRemoteImage returned no blob')
      }
      const createResponse = await createImageMutation.mutateAsync({
        image: {
          ownerType,
          ownerId,
          imageType,
          url: blob.key,
          width: blob.width,
          height: blob.height,
          source,
          priority: priority ?? 0,
        },
      })
      if (!createResponse.image) {
        throw new Error('CreateImage returned no image')
      }
      return createResponse.image
    },
    [cacheRemoteImageMutation, createImageMutation],
  )

  const attachFromFile = useCallback(
    async (params: AttachFromFileParams): Promise<Image> => {
      const { data, ownerType, ownerId, imageType, priority } = params
      const uploadResponse = await uploadImageMutation.mutateAsync({ data })
      const blob = uploadResponse.blob
      if (!blob) {
        throw new Error('UploadImage returned no blob')
      }
      const createResponse = await createImageMutation.mutateAsync({
        image: {
          ownerType,
          ownerId,
          imageType,
          url: blob.key,
          width: blob.width,
          height: blob.height,
          source: 'user',
          priority: priority ?? 0,
        },
      })
      if (!createResponse.image) {
        throw new Error('CreateImage returned no image')
      }
      return createResponse.image
    },
    [uploadImageMutation, createImageMutation],
  )

  return {
    attachFromUrl,
    attachFromFile,
    cacheRemoteImage: cacheRemoteImageMutation,
    uploadImage: uploadImageMutation,
    createImage: createImageMutation,
  }
}
