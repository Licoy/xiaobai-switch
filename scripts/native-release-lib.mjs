import { createHash, createPrivateKey, createPublicKey, sign, verify } from 'node:crypto'
import path from 'node:path'

export const targets = ['darwin-amd64','darwin-arm64','windows-amd64','windows-arm64','linux-amd64','linux-arm64']
export function version(value) {
 const match=/^([1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/.exec(value??'')
 if(!match||match[4]?.split('.').some(part=>/^0[0-9]+$/.test(part)))throw new Error('RELEASE_VERSION must be native semver >=1.0.0 without v')
 return value
}
export function assetName(v,target,suffix='tar.gz') {
 version(v)
 if (!targets.includes(target)||!['tar.gz','app.tar.gz','dmg','zip','setup.exe'].includes(suffix)) throw new Error('invalid asset target or suffix')
 return `XiaoBaiSwitch_${v}_${target}${suffix==='setup.exe'?'-setup.exe':'.'+suffix}`
}
export function secureURL(raw) {
 const u = new URL(raw)
 if(u.protocol!=='https:'||u.username||u.password||u.hash)throw new Error('release URL requires credential-free HTTPS')
 return u.href
}
export function safeEntry(entry) {
 if(!entry || path.posix.normalize(entry).replace(/\/+$/,'')==='.' || entry.includes('\\') || entry.startsWith('/') || /^[a-z]:/i.test(entry) || entry.split('/').includes('..')) throw new Error('unsafe archive entry')
 return entry
}
export function decodeKey(raw,n) {
 if(typeof raw!=='string'||!/^[A-Za-z0-9+/]+={0,2}$/.test(raw))throw new Error('invalid base64 signing key')
 const b=Buffer.from(raw,'base64')
 if(b.length!==n || b.toString('base64')!==raw)throw new Error(`signing key must contain ${n} bytes`)
 return b
}
export function signingKeys(publicText,privateText) {
 if(!publicText)throw new Error('MYGO_UPDATER_PUBLIC_KEY is required')
 if(!privateText)throw new Error('MYGO_UPDATER_PRIVATE_KEY is required')
 const pub=decodeKey(publicText,32), priv=decodeKey(privateText,64)
 const privateKey=createPrivateKey({key:Buffer.concat([Buffer.from('302e020100300506032b657004220420','hex'),priv.subarray(0,32)]),format:'der',type:'pkcs8'})
 const actual=createPublicKey(privateKey).export({format:'der',type:'spki'}).subarray(-32)
 if(!actual.equals(pub)||!priv.subarray(32).equals(pub)) throw new Error('native signing keys do not match')
 return {privateKey,publicKey:createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),pub]),format:'der',type:'spki'})}
}
export function nativeManifest({v,target,data,baseURL,keys,notes='',date=new Date().toISOString()}) {
 const digest=createHash('sha256').update(data).digest()
 const signature=sign(null,digest,keys.privateKey).toString('base64')
 if(!verify(null,digest,keys.publicKey,Buffer.from(signature,'base64')))throw new Error('signature verification failed')
 return {version:version(v),notes,date,url:secureURL(`${baseURL}/${assetName(v,target)}`),size:data.length,signature}
}
export function validateManifest(m,{v,target,publicKey}) {
 if(m.version!==version(v)||!Number.isSafeInteger(m.size)||m.size<=0||m.size>1024**3||typeof m.date!=='string'||!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(m.date)||!Number.isFinite(Date.parse(m.date)))throw new Error('invalid update metadata')
 const u=new URL(secureURL(m.url))
 if(decodeURIComponent(path.posix.basename(u.pathname))!==assetName(v,target))throw new Error('update archive does not match target/version')
 decodeKey(m.signature,64);decodeKey(publicKey,32)
 return m
}
const legacy={ 'darwin-aarch64':'darwin-arm64','darwin-x86_64':'darwin-amd64','windows-aarch64':'windows-arm64','windows-x86_64':'windows-amd64' }
export function transitionManifest({v,assets,notes='',date=new Date().toISOString()}){
 const platforms={}
 for(const [old,target] of Object.entries(legacy)){
  const name=assetName(v,target,target.startsWith('darwin')?'app.tar.gz':'setup.exe')
  const a=assets.find(a=>a.name===name)
  if(!a||!a.signature||!a.url)throw new Error(`missing signed bridge asset ${name}`)
  const text=Buffer.from(a.signature.trim(),'base64').toString('utf8')
  const lines=text.trim().split(/\r?\n/)
  if(lines.length!==4||!lines[0].startsWith('untrusted comment:')||!lines[2].startsWith('trusted comment:'))throw new Error('Tauri requires a base64 minisign signature file')
  decodeKey(lines[1],74);decodeKey(lines[3],64)
  platforms[old]={url:secureURL(a.url),signature:a.signature.trim()}
  platforms[`${old}-${target.startsWith('darwin')?'app':'nsis'}`]=platforms[old]
 }
 return {version:version(v),notes,pub_date:date,platforms}
}
export function immutableBridgeManifest(raw){
 if(raw?.version!=='0.0.12'||!raw.platforms)throw new Error('immutable bridge must be 0.0.12')
 const required=['darwin-aarch64','darwin-aarch64-app','darwin-x86_64','darwin-x86_64-app','windows-aarch64','windows-aarch64-nsis','windows-x86_64','windows-x86_64-msi','windows-x86_64-nsis']
 for(const key of required)if(!raw.platforms[key])throw new Error(`bridge missing ${key}`)
 for(const [key,p] of Object.entries(raw.platforms)){
  if(!required.includes(key)||!p?.signature)throw new Error(`invalid bridge platform ${key}`)
  const u=new URL(secureURL(p.url));if(!u.pathname.includes('/releases/download/v0.0.12/'))throw new Error('bridge URL must name immutable v0.0.12 asset')
 }

 return raw
}

export function verifyTauriSignature(data,signature,publicText){
 const pubLines=Buffer.from(publicText,'base64').toString('utf8').trim().split(/\r?\n/)
 if(pubLines.length!==2)throw new Error('invalid Tauri public key')
 const pub=decodeKey(pubLines[1],42)
 const lines=Buffer.from(signature.trim(),'base64').toString('utf8').trim().split(/\r?\n/)
 if(lines.length!==4||!lines[2].startsWith('trusted comment: '))throw new Error('invalid minisign signature')
 const packet=decodeKey(lines[1],74),global=decodeKey(lines[3],64)
 if(packet.subarray(0,2).toString()!=='ED'||!pub.subarray(2,10).equals(packet.subarray(2,10)))throw new Error('minisign algorithm or key ID mismatch')
 const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),pub.subarray(10)]),format:'der',type:'spki'})
 const digest=createHash('blake2b512').update(data).digest(),sig=packet.subarray(10)
 if(!verify(null,digest,key,sig)||!verify(null,Buffer.concat([sig,Buffer.from(lines[2].slice(17))]),key,global))throw new Error('Tauri minisign verification failed')
 return true
}
