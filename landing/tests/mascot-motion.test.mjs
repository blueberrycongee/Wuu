import test from 'node:test';
import assert from 'node:assert/strict';
import { eyePose, projectEye, spinAngle, springStep, blinkLid } from '../mascot-motion.mjs';
const eyes = [{ x: 46.5, y: 56.7 }, { x: 64.5, y: 56 }];

test('spherical projection preserves the authored eyes at rest and after a turn', () => {
  for (const eye of eyes) for (const angle of [0, Math.PI * 2]) {
    const pose = projectEye(eye, angle);
    assert.ok(Math.abs(pose.x - eye.x) < 1e-10);
    assert.ok(Math.abs(pose.y - eye.y) < 1e-10);
    assert.ok(Math.abs(pose.width - 1) < 1e-10);
    assert.equal(pose.opacity, 1);
  }
});

test('eyes cross the silhouette independently rather than disappearing as one sticker', () => {
  const left = projectEye(eyes[0], Math.PI / 2);
  const right = projectEye(eyes[1], Math.PI / 2);
  assert.ok(left.opacity > 0);
  assert.equal(right.opacity, 0);
  assert.ok(left.width > right.width);
  for (const eye of eyes) assert.equal(projectEye(eye, Math.PI).opacity, 0);
});

test('projection remains continuous across both silhouette edges', () => {
  for (const eye of eyes) {
    let previous = projectEye(eye, 0);
    for (let angle = 0.001; angle < Math.PI * 2; angle += 0.001) {
      const current = projectEye(eye, angle);
      assert.ok(Math.abs(current.opacity - previous.opacity) < 0.013);
      assert.ok(Math.abs(current.x - previous.x) < 0.04);
      previous = current;
    }
  }
});

test('turn anticipates and overshoots but finishes at exactly one revolution', () => {
  assert.ok(Math.abs(spinAngle(0)) < 1e-10);
  assert.ok(spinAngle(0.1) < 0);
  assert.ok(spinAngle(0.8) > Math.PI * 2);
  assert.equal(spinAngle(1), Math.PI * 2);
  for (const join of [0.14, 0.78]) assert.ok(Math.abs(spinAngle(join - 1e-6) - spinAngle(join + 1e-6)) < 1e-6);
});

test('glances release into the existing gaze without moving its anchor', () => {
  const gaze = { x: 7, y: -3 };
  for (const action of ['peek', 'wink', 'spin', 'bright', 'squint', 'scan']) {
    const end = eyePose(0, action, 1, gaze);
    assert.ok(Math.abs(end.x - gaze.x) < 1e-10);
    assert.ok(Math.abs(end.y - gaze.y) < 1e-10);
    assert.ok(Math.abs(end.roll) < 1e-10);
  }
});

test('gaze spring settles consistently across display frame rates', () => {
  for (const fps of [30, 60, 144]) {
    let value = 0, velocity = 0;
    for (let i = 0; i < fps; i++) ({ value, velocity } = springStep(value, velocity, 10, 1 / fps));
    assert.ok(Math.abs(value - 10) < 0.001);
    assert.ok(Math.abs(velocity) < 0.01);
  }
});


test('blinks close fully, hold briefly and reopen more slowly', () => {
  assert.equal(blinkLid(0), 1);
  assert.equal(blinkLid(0.3), 0.02);
  assert.equal(blinkLid(0.4), 0.02);
  assert.ok(blinkLid(0.65) < 0.5);
  assert.equal(blinkLid(1), 1);
  assert.equal(blinkLid(2), 1);
});

test('new expressions visibly change eye shape and release it at the end', () => {
  const gaze = { x: 0, y: 0 };
  assert.ok(eyePose(0, 'bright', 0.35, gaze).scaleY > 1.2);
  assert.ok(eyePose(0, 'squint', 0.35, gaze).scaleY < 0.5);
  for (const action of ['bright', 'squint', 'scan']) {
    assert.equal(eyePose(0, action, 1, gaze).scaleY, 1);
    assert.equal(eyePose(0, action, 1, gaze).scaleX, 1);
  }
});
