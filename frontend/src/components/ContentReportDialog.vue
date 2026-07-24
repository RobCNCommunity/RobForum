<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppIcon from '@/components/AppIcon.vue'

const props = withDefaults(defineProps<{
  open: boolean
  targetLabel: string
  submitting?: boolean
}>(), { submitting: false })

const emit = defineEmits<{
  close: []
  submit: [reason: string]
}>()

const reason = ref('')
const ready = computed(() => reason.value.trim().length >= 2)

watch(() => props.open, (open) => {
  if (open) reason.value = ''
})

function close() {
  if (!props.submitting) emit('close')
}

function submit() {
  if (ready.value && !props.submitting) emit('submit', reason.value.trim())
}
</script>

<template>
  <div v-if="open" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="close">
    <section class="rf-dialog rf-report-dialog" role="dialog" aria-modal="true" aria-label="举报内容">
      <header>
        <div><h2>举报</h2><p>{{ targetLabel }}</p></div>
        <button type="button" class="rf-icon-button" aria-label="关闭" :disabled="submitting" @click="close"><AppIcon name="close" size="18" /></button>
      </header>
      <div class="rf-report-dialog-note"><AppIcon name="flag" size="18" /><span>审核结果会通过站内通知告知你。</span></div>
      <label class="rf-field-label"><span>举报原因</span><textarea v-model="reason" class="rf-control rf-textarea" rows="4" maxlength="500" placeholder="请说明具体违规原因" /></label>
      <footer><button type="button" class="rf-secondary-button" :disabled="submitting" @click="close">取消</button><nut-button type="danger" :loading="submitting" :disabled="!ready" @click="submit">提交举报</nut-button></footer>
    </section>
  </div>
</template>

<style scoped>
.rf-report-dialog-note { display: flex; align-items: flex-start; gap: 9px; margin: 16px 20px 0; padding: 12px; border-radius: 8px; color: var(--rf-text-muted); background: var(--rf-bg-subtle); font-size: 13px; line-height: 1.5; }
.rf-report-dialog-note :deep(svg) { flex: 0 0 auto; color: var(--rf-danger); }
</style>
