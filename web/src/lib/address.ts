import type { Item } from './types'

/** 按地址模式选择卡片链接；所选模式没有填写时回退到另一个。 */
export function itemUrl(it: Pick<Item, 'urlLan' | 'urlWan'>, mode: 'lan' | 'wan') {
  return mode === 'wan' ? it.urlWan || it.urlLan : it.urlLan || it.urlWan || ''
}
