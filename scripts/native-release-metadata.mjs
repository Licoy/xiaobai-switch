// Final publication checks run after both updater protocols have been assembled.
import fs from 'node:fs'
import path from 'node:path'
import {createHash} from 'node:crypto'
import {fileURLToPath} from 'node:url'
import {targets,version} from './native-release-lib.mjs'
export function releaseNotes(text) {
 if(typeof text!=='string'||!text.trim())throw new Error('release notes must not be empty')
 return text.trim()
}
export function finalize(directory,v) {
 version(v)
 const notes=releaseNotes(fs.readFileSync(path.join(directory,'release-notes.md'),'utf8'))
 for(const name of [...targets.map(t=>`update-${t}.json`),'native-transition.json']) {
  const manifest=JSON.parse(fs.readFileSync(path.join(directory,name),'utf8'))
  if(manifest.version!==v||manifest.notes!==notes)throw new Error(`release notes/version mismatch: ${name}`)
 }
 const names=fs.readdirSync(directory).filter(n=>n!=='SHA256SUMS').sort()
 const sums=names.map(name=>{
  if(!/^[A-Za-z0-9_.-]+$/.test(name))throw new Error('unsafe release asset filename')
  const file=path.join(directory,name),stat=fs.lstatSync(file)
  if(!stat.isFile()||stat.size===0)throw new Error(`invalid release asset: ${name}`)
  return `${createHash('sha256').update(fs.readFileSync(file)).digest('hex')}  ${name}\n`
 }).join('')
 fs.writeFileSync(path.join(directory,'SHA256SUMS'),sums)
 return sums
}
if(process.argv[1]===fileURLToPath(import.meta.url))finalize(process.env.NATIVE_RELEASE_DIR??'.build/native-release/assets',process.env.RELEASE_VERSION)
