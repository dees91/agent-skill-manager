import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'
import { STORYBOARD, VIDEO } from './storyboard'

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
