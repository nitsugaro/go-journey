import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'

const fixtureURL = new URL('../../test/config/auth/journeys/1ab0d0ef-e889-47a4-bda8-5c931fea4339.json', import.meta.url)

test('replaces an unavoidable crossing between adjacent branches with one shared reference', async (context) => {
  const vite = await createServer({
    configFile: fileURLToPath(new URL('../vite.config.ts', import.meta.url)),
    server: { middlewareMode: true },
    appType: 'custom',
    logLevel: 'silent',
  })
  context.after(() => vite.close())

  const journey = JSON.parse(await readFile(fixtureURL, 'utf8'))
  const { buildJourneyFlow } = await vite.ssrLoadModule('/src/pages/flow/journeyFlowLayout.ts')
  const flow = buildJourneyFlow(journey, {}, {
    endJourneyTypes: new Set(['HTTPFinishResponse']),
  })
  const scheduleID = '4765be04-d183-4372-8a34-4a45143cc8a2'
  const schemaID = '48c08504-d6a7-4eeb-9448-914eab31b742'
  const okResponseID = '3262efc9-681a-409e-9ebf-48af5ffb7c8c'
  const badResponseID = 'e8013bb3-dca6-47ca-8d14-652302d6939d'
  const crossingReferenceID = `ref:${scheduleID}:${badResponseID}`
  const positions = new Map(flow.nodes.map((node) => [node.id, node.position]))

  assert.ok(positions.get(scheduleID).y < positions.get(schemaID).y, 'the upper source branch must remain above the lower branch')
  assert.ok(positions.get(okResponseID).y < positions.get(badResponseID).y, 'targets must preserve the same vertical order as their sources')

  const scheduleEdges = flow.edges.filter((edge) => edge.source === scheduleID)
  assert.deepEqual(new Set(scheduleEdges.map((edge) => edge.target)), new Set([okResponseID, crossingReferenceID]))

  const schemaEdges = flow.edges.filter((edge) => edge.source === schemaID)
  assert.deepEqual(new Set(schemaEdges.map((edge) => edge.target)), new Set([okResponseID, badResponseID]))
  const crossingReferences = flow.nodes.filter((node) => node.id === crossingReferenceID && node.data?.reference && node.data?.originalId === badResponseID)
  assert.equal(crossingReferences.length, 1, 'only the downward connection that crosses the lower branch should become a reference')
  assert.ok(positions.get(crossingReferenceID).y >= 120, 'placing a reference must keep the canvas inside its top padding')

  const directEdges = flow.edges.filter((edge) => positions.has(edge.source) && positions.has(edge.target) && !edge.data?.reference)
  for (let leftIndex = 0; leftIndex < directEdges.length; leftIndex++) {
    for (let rightIndex = leftIndex + 1; rightIndex < directEdges.length; rightIndex++) {
      const left = directEdges[leftIndex]
      const right = directEdges[rightIndex]
      if (left.source === right.source || left.target === right.target) continue
      const leftSource = positions.get(left.source)
      const rightSource = positions.get(right.source)
      const leftTarget = positions.get(left.target)
      const rightTarget = positions.get(right.target)
      if (Math.abs(leftSource.x - rightSource.x) > 160 || Math.abs(leftTarget.x - rightTarget.x) > 160) continue
      assert.ok(
        (leftSource.y - rightSource.y) * (leftTarget.y - rightTarget.y) >= 0,
        `direct edges ${left.id} and ${right.id} must not reverse their vertical order`,
      )
    }
  }

  const cookieReferences = flow.nodes.filter((node) => node.data?.reference && node.data?.originalId === '3262efc9-681a-409e-9ebf-48af5ffb7c8c' && node.id.includes('67cd0071-5938-4d71-ac77-8f8ae5f13f57'))
  assert.equal(cookieReferences.length, 1, 'outcomes from one source to the same target must share one reference node')
})
