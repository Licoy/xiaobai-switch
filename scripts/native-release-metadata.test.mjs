import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {spawnSync,execFileSync} from 'node:child_process'
import {notesRange} from './native-release-notes-range.mjs'
import {createHash} from 'node:crypto'
import {targets} from './native-release-lib.mjs'
import {releaseNotes,finalize} from './native-release-metadata.mjs'
import {redact} from './native-release-log.mjs'
test('publication requires consistent nonempty notes and hashes every asset',()=>{
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'native-metadata-'))
 try {
  assert.throws(()=>releaseNotes(' \n'))
  fs.writeFileSync(path.join(dir,'release-notes.md'),' Changes\n')
  for(const name of [...targets.map(t=>`update-${t}.json`),'native-transition.json'])fs.writeFileSync(path.join(dir,name),JSON.stringify({version:'1.0.0',notes:'Changes'}))
  fs.writeFileSync(path.join(dir,'darwin-arm64.log'),'verified smoke')
  const sums=finalize(dir,'1.0.0')
  assert.equal(sums.split('\n').filter(Boolean).length,9)
  for(const line of sums.trimEnd().split('\n')){
   const [digest,name]=line.split('  ')
   assert.equal(digest,createHash('sha256').update(fs.readFileSync(path.join(dir,name))).digest('hex'))
  }
  assert.equal(finalize(dir,'1.0.0'),sums)
  fs.writeFileSync(path.join(dir,'update-linux-arm64.json'),JSON.stringify({version:'1.0.0',notes:''}))
  assert.throws(()=>finalize(dir,'1.0.0'),/notes\/version mismatch/)
 } finally {fs.rmSync(dir,{recursive:true,force:true})}
})
test('downloadable logs redact secrets across chunks and preserve command failures',()=>{
 assert.equal(redact('token-long token',{GH_TOKEN:'token',PRIVATE_KEY:'token-long'}),'[REDACTED] [REDACTED]')
 assert.equal(redact('second-line\n',{PRIVATE_KEY:'first-line\nsecond-line'}),'[REDACTED]\n')
 const multiline={PRIVATE_KEY:'sensitive\nsecond-line'}
 const expected='[REDACTED]\n[REDACTED]\n'
 assert.equal(redact('sensitive\nsecond-line\n',multiline),expected)
 assert.equal(['sensitive\n','second-line\n'].map(line=>redact(line,multiline)).join(''),expected)
 assert.equal(redact('first\r\n\r\nlast\r\n',{PRIVATE_KEY:'first\r\n\r\nlast'}),'[REDACTED]\r\n\r\n[REDACTED]\r\n')
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'native-log-'))
 try {
  const file=path.join(dir,'build.log')
  const result=spawnSync(process.execPath,['scripts/native-release-log.mjs',file,process.execPath,'-e',"process.stdout.write('sensi');setTimeout(()=>{console.log('tive');console.error('sensitive');console.log('second-line');process.exitCode=7},20)"],{encoding:'utf8',env:{...process.env,MYGO_UPDATER_PRIVATE_KEY:'sensitive\nsecond-line'}})
  assert.equal(result.status,7)
  assert.equal(result.stdout,expected)
  assert.equal(result.stderr,'[REDACTED]\n')
  assert(!result.stdout.includes('sensitive'))
  assert(!result.stderr.includes('sensitive'))
  assert(!result.stdout.includes('second-line'))
  assert.equal(fs.readFileSync(file,'utf8').split('\n').filter(Boolean).length,3)
  assert(!fs.readFileSync(file,'utf8').includes('second-line'))
 } finally {fs.rmSync(dir,{recursive:true,force:true})}
})

test('commit notes range excludes published history and future tags, stable includes prerelease work',()=>{
 const cwd=fs.mkdtempSync(path.join(os.tmpdir(),'native-notes-git-'))
 const git=(...args)=>execFileSync('git',args,{cwd,encoding:'utf8',stdio:['ignore','pipe','pipe']}).trim()
 const commit=message=>{git('-c','user.name=Fixture','-c','user.email=fixture@example.test','commit','--allow-empty','-m',message);return git('rev-parse','HEAD')}
 try {
  git('init','--quiet')
  const initial=commit('feat: initial release');git('tag','v1.0.0')
  assert.equal(notesRange('1.0.0',cwd),initial)
  git('tag','-d','v1.0.0')
  const legacy=commit('feat: legacy release');git('tag','v0.0.11')
  commit('feat: complete native rebuild');git('tag','v0.0.12')
  const old=commit('fix: native release readiness');git('tag','v1.0.0')
  const first=notesRange('1.0.0',cwd)
  assert.equal(first,`${legacy}..${old}`)
  const firstMessages=git('log','--format=%s',first)
  assert(firstMessages.includes('complete native rebuild'))
  assert(firstMessages.includes('native release readiness'))
  assert(!firstMessages.includes('legacy release'))
  const beta=commit('feat: beta improvement');git('tag','v1.1.0-beta.1')
  assert.equal(notesRange('1.1.0-beta.1',cwd),`${old}..${beta}`)
  const current=commit('fix: current release');git('tag','v1.1.0');git('tag','v1.1.0-rc.1')
  commit('feat: future release');git('tag','v2.0.0')
  const range=notesRange('1.1.0',cwd)
  assert.equal(range,`${old}..${current}`)
  const messages=git('log','--format=%s',range)
  assert(messages.includes('beta improvement'))
  assert(messages.includes('current release'))
  assert(!messages.includes('native release readiness'))
  assert(!messages.includes('complete native rebuild'))
  assert(!messages.includes('future release'))
 } finally {fs.rmSync(cwd,{recursive:true,force:true})}
})
