// Assemble publication metadata only after signed installers and all six feeds exist.
import fs from 'node:fs'
import path from 'node:path'
import {createHash,createPublicKey,verify} from 'node:crypto'
import {fileURLToPath} from 'node:url'
import {targets,version,assetName,secureURL,transitionManifest,immutableBridgeManifest,validateManifest,verifyTauriSignature} from './native-release-lib.mjs'
export function assemble({directory,v,baseURL,bridgeFile,publicKey,tauriPublicKey,notes=''}){
 version(v);secureURL(baseURL)
 const assets=[]
 const tauriKey=tauriPublicKey??JSON.parse(fs.readFileSync('src-tauri/tauri.conf.json','utf8')).plugins.updater.pubkey
 for(const target of targets){
  const name=assetName(v,target),feed=JSON.parse(fs.readFileSync(path.join(directory,`update-${target}.json`),'utf8'))
  validateManifest(feed,{v,target,publicKey})
  const payload=fs.readFileSync(path.join(directory,name)),pub=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from(publicKey,'base64')]),format:'der',type:'spki'})
  if(!verify(null,createHash('sha256').update(payload).digest(),pub,Buffer.from(feed.signature,'base64')))throw new Error(`invalid native signature ${name}`)
  if(payload.length!==feed.size)throw new Error(`archive size mismatch ${name}`)
  if(target.startsWith('linux')){if(!fs.statSync(path.join(directory,'install.sh')).size)throw new Error('missing Linux installer');continue}
  const download=assetName(v,target,target.startsWith('darwin')?'dmg':'zip')
  if(!fs.statSync(path.join(directory,download)).size)throw new Error(`empty download ${download}`)
  const installer=assetName(v,target,target.startsWith('darwin')?'app.tar.gz':'setup.exe')
  if(fs.statSync(path.join(directory,installer)).size===0)throw new Error(`empty installer ${installer}`)
  verifyTauriSignature(fs.readFileSync(path.join(directory,installer)),fs.readFileSync(path.join(directory,installer+'.sig'),'utf8'),tauriKey)
  assets.push({name:installer,url:`${baseURL}/${installer}`,signature:fs.readFileSync(path.join(directory,installer+'.sig'),'utf8')})
 }
 const legacy=immutableBridgeManifest(JSON.parse(fs.readFileSync(bridgeFile,'utf8')))
 const transition=transitionManifest({v,assets,notes})
 fs.writeFileSync(path.join(directory,'latest.json'),JSON.stringify(legacy,null,2)+'\n')
 fs.writeFileSync(path.join(directory,'native-transition.json'),JSON.stringify(transition,null,2)+'\n')
 return transition
}
if(process.argv[1]===fileURLToPath(import.meta.url))assemble({directory:process.env.NATIVE_RELEASE_DIR??'.build/native-release/assets',v:process.env.RELEASE_VERSION,baseURL:`https://github.com/Licoy/xiaobai-switch/releases/download/v${process.env.RELEASE_VERSION}`,bridgeFile:process.env.NATIVE_BRIDGE_MANIFEST,publicKey:process.env.MYGO_UPDATER_PUBLIC_KEY,notes:process.env.RELEASE_NOTES??''})
