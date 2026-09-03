import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'
import { SOCIAL_PREVIEW, STORYBOARD, VIDEO } from './storyboard'

test('storyboard covers one continuous overlapping 20 second composition', () => {
  assert.equal(VIDEO.durationInFrames, VIDEO.fps * 20)
  assert.equal(STORYBOARD[0]?.start, 0)
  assert.equal(STORYBOARD.at(-1)?.end, VIDEO.durationInFrames)
  assert.equal(new Set(STORYBOARD.map((scene) => scene.id)).size, STORYBOARD.length)

  STORYBOARD.forEach((scene, index) => {
    assert.ok(Number.isInteger(scene.start) && Number.isInteger(scene.end))
    assert.ok(scene.start >= 0 && scene.end > scene.start)
    if (index > 0) {
      const previous = STORYBOARD[index - 1]
      assert.ok(scene.start <= previous.end, `${scene.id} leaves a timeline gap`)
      assert.ok(scene.start >= previous.start, `${scene.id} is out of order`)
    }
  })
})

test('declared assets exist in the tracked public directory', () => {
  for (const scene of STORYBOARD) {
    for (const asset of scene.assets ?? []) {
      assert.ok(existsSync(resolve(process.cwd(), 'public', asset)), `${scene.id} is missing public/${asset}`)
    }
  }
})

test('all storyboard copy is populated', () => {
  for (const scene of STORYBOARD) {
    for (const [key, value] of Object.entries(scene.copy)) {
      assert.ok(typeof value === 'string' ? value.trim().length > 0 : value.length > 0, `${scene.id}.${key} is empty`)
    }
  }
})

test('every managed tool appears in the scattered-skills and closing copy', () => {
  const problem = STORYBOARD.find((scene) => scene.id === 'problem')
  assert.ok(problem)
  for (const key of ['claudePath', 'codexPath', 'musePath', 'grokPath']) {
    assert.ok(typeof problem.copy[key] === 'string', `problem copy is missing ${key}`)
  }
  for (const tool of ['Claude Code', 'Codex', 'Muse', 'Grok']) {
    assert.ok(SOCIAL_PREVIEW.copy.supporting.includes(tool), `social preview omits ${tool}`)
    assert.ok(String(STORYBOARD.at(-1)?.copy.footer).includes(tool), `closing footer omits ${tool}`)
  }
})

test('social preview has stable GitHub dimensions, copy, and tracked assets', () => {
  assert.equal(SOCIAL_PREVIEW.width, 1280)
  assert.equal(SOCIAL_PREVIEW.height, 640)
  assert.equal(SOCIAL_PREVIEW.copy.headline, 'Agent Skills, under control.')

  for (const value of Object.values(SOCIAL_PREVIEW.copy)) {
    assert.ok(value.trim().length > 0)
  }

  assert.deepEqual([...SOCIAL_PREVIEW.tools], ['claude', 'codex', 'muse', 'grok'])
  assert.equal(SOCIAL_PREVIEW.rows.length, 3)
  for (const row of SOCIAL_PREVIEW.rows) {
    assert.ok(row.name.trim().length > 0)
    for (const tool of SOCIAL_PREVIEW.tools) {
      assert.ok(['ON', 'OFF'].includes(row.states[tool]), `${row.name}.${tool} is not a visibility state`)
    }
  }

  assert.ok(existsSync(resolve(process.cwd(), 'public', SOCIAL_PREVIEW.asset)))
})
