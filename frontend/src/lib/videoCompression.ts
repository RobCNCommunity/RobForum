export const COMMUNITY_IMAGE_MAX_BYTES = 5 * 1024 * 1024
export const COMMUNITY_VIDEO_MAX_BYTES = 50 * 1024 * 1024
export const COMMUNITY_VIDEO_SOURCE_MAX_BYTES = 250 * 1024 * 1024

const VIDEO_COMPRESSION_THRESHOLD_BYTES = 8 * 1024 * 1024
const VIDEO_TARGET_BYTES = 44 * 1024 * 1024
const MAX_VIDEO_BITRATE = 2_500_000
const MIN_VIDEO_BITRATE = 160_000
const MAX_VIDEO_EDGE = 1280

const imageTypes = new Set(['image/png', 'image/jpeg'])
const videoTypes = new Set(['video/mp4', 'video/webm'])

export interface PreparedCommunityMedia {
  file: File
  compressed: boolean
  originalSize: number
}

export function isCommunityImageFile(file: File) {
  return imageTypes.has(file.type) && /\.(png|jpe?g)$/i.test(file.name)
}

export function isCommunityVideoFile(file: File) {
  return videoTypes.has(file.type) && /\.(mp4|webm)$/i.test(file.name)
}

function abortError() {
  return new DOMException('视频压缩已取消', 'AbortError')
}

function outputName(file: File, extension: 'mp4' | 'webm') {
  const baseName = file.name.replace(/\.[^.]+$/, '') || 'video'
  return `${baseName}-compressed.${extension}`
}

function compressionBitrates(duration: number, hasAudio: boolean) {
  const totalBitrate = Math.floor((VIDEO_TARGET_BYTES * 8 * 0.88) / duration)
  const audioBitrate = hasAudio
    ? Math.min(128_000, Math.max(64_000, Math.floor(totalBitrate * 0.15)))
    : 0
  return {
    audioBitrate,
    videoBitrate: Math.min(MAX_VIDEO_BITRATE, totalBitrate - audioBitrate),
  }
}

async function compressVideo(
  file: File,
  onProgress: ((progress: number) => void) | undefined,
  signal: AbortSignal | undefined,
) {
  if (signal?.aborted) throw abortError()
  if (typeof VideoDecoder === 'undefined' || typeof VideoEncoder === 'undefined') {
    throw new Error('当前浏览器不支持视频压缩')
  }

  const {
    ALL_FORMATS,
    BlobSource,
    BufferTarget,
    Conversion,
    Input,
    Mp4OutputFormat,
    Output,
    WebMOutputFormat,
  } = await import('mediabunny')

  const probe = new Input({ source: new BlobSource(file), formats: ALL_FORMATS })
  let duration = 0
  let hasAudio = false
  try {
    if (!await probe.canRead()) throw new Error('无法读取这个视频文件')
    const videoTrack = await probe.getPrimaryVideoTrack()
    if (!videoTrack) throw new Error('文件中没有可播放的视频轨道')
    hasAudio = Boolean(await probe.getPrimaryAudioTrack())
    const metadataDuration = await probe.getDurationFromMetadata()
    duration = metadataDuration && Number.isFinite(metadataDuration)
      ? metadataDuration
      : await probe.computeDuration()
  } finally {
    probe.dispose()
  }

  if (!Number.isFinite(duration) || duration <= 0) throw new Error('无法获取视频时长')
  const { audioBitrate, videoBitrate } = compressionBitrates(duration, hasAudio)
  if (videoBitrate < MIN_VIDEO_BITRATE) {
    throw new Error('视频时长过长，无法在 50 MB 内保持可用画质')
  }

  const candidates = file.type === 'video/webm'
    ? [
        { extension: 'webm' as const, videoCodec: 'vp9' as const, audioCodec: 'opus' as const },
        { extension: 'mp4' as const, videoCodec: 'avc' as const, audioCodec: 'aac' as const },
      ]
    : [
        { extension: 'mp4' as const, videoCodec: 'avc' as const, audioCodec: 'aac' as const },
        { extension: 'webm' as const, videoCodec: 'vp9' as const, audioCodec: 'opus' as const },
      ]

  let lastError: unknown
  for (const candidate of candidates) {
    if (signal?.aborted) throw abortError()
    const input = new Input({ source: new BlobSource(file), formats: ALL_FORMATS })
    const target = new BufferTarget()
    const output = new Output({
      format: candidate.extension === 'mp4' ? new Mp4OutputFormat() : new WebMOutputFormat(),
      target,
    })
    let removeAbortListener: (() => void) | undefined

    try {
      const primaryVideo = await input.getPrimaryVideoTrack()
      const primaryAudio = await input.getPrimaryAudioTrack()
      if (!primaryVideo) throw new Error('文件中没有可播放的视频轨道')

      const conversion = await Conversion.init({
        input,
        output,
        tracks: 'primary',
        video: async (track) => {
          const width = await track.getDisplayWidth()
          const height = await track.getDisplayHeight()
          return {
            ...(width >= height ? { width: Math.min(width, MAX_VIDEO_EDGE) } : { height: Math.min(height, MAX_VIDEO_EDGE) }),
            frameRate: 30,
            codec: candidate.videoCodec,
            bitrate: videoBitrate,
            forceTranscode: true,
          }
        },
        audio: {
          codec: candidate.audioCodec,
          bitrate: audioBitrate || undefined,
          forceTranscode: true,
        },
      })
      if (signal?.aborted) throw abortError()

      const preservesVideo = conversion.utilizedTracks.includes(primaryVideo)
      const preservesAudio = !primaryAudio || conversion.utilizedTracks.includes(primaryAudio)
      if (!conversion.isValid || !preservesVideo || !preservesAudio) {
        throw new Error('浏览器不支持这个视频的编解码格式')
      }

      conversion.onProgress = (progress) => onProgress?.(Math.min(1, Math.max(0, progress)))
      if (signal) {
        const cancel = () => void conversion.cancel()
        signal.addEventListener('abort', cancel, { once: true })
        removeAbortListener = () => signal.removeEventListener('abort', cancel)
      }

      await conversion.execute()
      if (signal?.aborted) throw abortError()
      if (!target.buffer) throw new Error('视频压缩没有生成输出文件')

      onProgress?.(1)
      return new File([target.buffer], outputName(file, candidate.extension), {
        type: candidate.extension === 'mp4' ? 'video/mp4' : 'video/webm',
        lastModified: Date.now(),
      })
    } catch (error) {
      if (signal?.aborted) throw abortError()
      lastError = error
    } finally {
      removeAbortListener?.()
      input.dispose()
    }
  }

  throw lastError instanceof Error ? lastError : new Error('视频压缩失败')
}

export async function prepareCommunityMediaFile(
  file: File,
  onProgress?: (progress: number) => void,
  signal?: AbortSignal,
): Promise<PreparedCommunityMedia> {
  if (isCommunityImageFile(file)) {
    if (file.size > COMMUNITY_IMAGE_MAX_BYTES) throw new Error('图片不能超过 5 MB')
    return { file, compressed: false, originalSize: file.size }
  }
  if (!isCommunityVideoFile(file)) throw new Error('仅支持 PNG、JPG、MP4 或 WebM 文件')
  if (file.size > COMMUNITY_VIDEO_SOURCE_MAX_BYTES) throw new Error('待压缩视频不能超过 250 MB')
  if (file.size <= VIDEO_COMPRESSION_THRESHOLD_BYTES) {
    return { file, compressed: false, originalSize: file.size }
  }

  try {
    const compressed = await compressVideo(file, onProgress, signal)
    if (compressed.size > COMMUNITY_VIDEO_MAX_BYTES) {
      if (file.size <= COMMUNITY_VIDEO_MAX_BYTES) return { file, compressed: false, originalSize: file.size }
      throw new Error('压缩后的视频仍超过 50 MB')
    }
    if (compressed.size >= file.size && file.size <= COMMUNITY_VIDEO_MAX_BYTES) {
      return { file, compressed: false, originalSize: file.size }
    }
    return { file: compressed, compressed: true, originalSize: file.size }
  } catch (error) {
    if (signal?.aborted) throw abortError()
    if (file.size <= COMMUNITY_VIDEO_MAX_BYTES) {
      onProgress?.(1)
      return { file, compressed: false, originalSize: file.size }
    }
    const reason = error instanceof Error ? error.message : '未知错误'
    throw new Error(`视频超过 50 MB，且无法压缩：${reason}`)
  }
}
