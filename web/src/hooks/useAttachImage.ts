import { useMutation } from '@connectrpc/connect-query'
import { useCallback } from 'react'
import {
  cacheRemoteImage,
  uploadImage,
} from '../gen/purser/domain/v1/image_blob-ImageBlobService_connectquery'
import { createImage, selectImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
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

// useAttachImage is the one place the three-call sequence — blob RPC,
// then ImageService.CreateImage, then ImageService.SelectImage — is
// written, so every consuming screen (Person photo, Artist poster, Album
// cover, ...) reuses it instead of re-deriving the call order itself.
// Attaching *is* the user picking: there's no separate step where a
// freshly attached image sits around unselected, so SelectImage always
// follows a successful CreateImage here rather than being left to each
// caller to remember.
//
// Deliberately returns the four underlying connect-query mutation
// objects (cacheRemoteImage/uploadImage/createImage/selectImage) rather
// than collapsing them into one status: a CacheRemoteImage/UploadImage
// failure leaves nothing orphaned (nothing was ever created), a
// CreateImage failure leaves an orphaned blob per
// docs/technical/image-caching-and-serving.md's accepted-risk note, and a
// SelectImage failure leaves a real, valid Image row that simply isn't
// the owner's current one yet — the UI needs to tell those apart, so the
// state stays per-call, same one-mutation-object-per-RPC shape
// useSettings.ts already established.
export function useAttachImage() {
  const cacheRemoteImageMutation = useMutation(cacheRemoteImage)
  const uploadImageMutation = useMutation(uploadImage)
  const createImageMutation = useMutation(createImage)
  const selectImageMutation = useMutation(selectImage)

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
      const image = createResponse.image
      if (!image) {
        throw new Error('CreateImage returned no image')
      }
      await selectImageMutation.mutateAsync({ ownerType, ownerId, imageType, imageId: image.id })
      return image
    },
    [cacheRemoteImageMutation, createImageMutation, selectImageMutation],
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
      const image = createResponse.image
      if (!image) {
        throw new Error('CreateImage returned no image')
      }
      await selectImageMutation.mutateAsync({ ownerType, ownerId, imageType, imageId: image.id })
      return image
    },
    [uploadImageMutation, createImageMutation, selectImageMutation],
  )

  return {
    attachFromUrl,
    attachFromFile,
    cacheRemoteImage: cacheRemoteImageMutation,
    uploadImage: uploadImageMutation,
    createImage: createImageMutation,
    selectImage: selectImageMutation,
  }
}
