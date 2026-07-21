export const postTypes = [
  { value: 'discussion', label: '讨论', description: '分享观点与社区见闻', icon: 'message' },
  { value: 'guide', label: '攻略', description: '发布玩法心得与教程', icon: 'star' },
  { value: 'resource', label: '资源', description: '介绍地图、素材或工具', icon: 'shop' },
  { value: 'team-up', label: '组队', description: '寻找一起游玩的队友', icon: 'people' },
  { value: 'recruitment', label: '招募', description: '招募开发者或项目成员', icon: 'add' },
  { value: 'trade', label: '交易', description: '发布合规的交易信息', icon: 'repost' },
] as const

const labels = new Map<string, string>(postTypes.map((item) => [item.value, item.label]))

export function postTypeLabel(value: string) {
  return labels.get(value) || value || '讨论'
}
