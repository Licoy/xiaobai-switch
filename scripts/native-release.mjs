import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import {spawnSync} from 'node:child_process'
import {fileURLToPath} from 'node:url'
import {run,packageWindows,packageMac} from './native-release-package.mjs'
import {releaseNotes} from './native-release-metadata.mjs'
import {validateUpdaterSigningSecrets} from './validate-updater-signing-secret.mjs'
import {assetName,version,signingKeys,nativeManifest,targets,safeEntry} from './native-release-lib.mjs'
export function preflight(env,target){
 version(env.RELEASE_VERSION)
 releaseNotes(env.RELEASE_NOTES)
 if(!targets.includes(target))throw new Error('unsupported release target')
 const keys=signingKeys(env.MYGO_UPDATER_PUBLIC_KEY,env.MYGO_UPDATER_PRIVATE_KEY)
 for(const name of ['TAURI_SIGNING_PRIVATE_KEY','TAURI_SIGNING_PRIVATE_KEY_PASSWORD'])if(!(name in env)||!env[name])throw new Error(`missing ${name}`)
 validateUpdaterSigningSecrets({privateKey:env.TAURI_SIGNING_PRIVATE_KEY,password:env.TAURI_SIGNING_PRIVATE_KEY_PASSWORD})
 if(target.startsWith('darwin'))for(const name of ['APPLE_SIGNING_IDENTITY','APPLE_SIGNING_CERT_SHA1'])if(!env[name])throw new Error(`missing ${name}`)
 if(target.startsWith('windows'))for(const name of ['WINDOWS_SIGN_CERT_FILE','WINDOWS_SIGN_CERT_PASSWORD','WINDOWS_SIGN_CERT_SHA1'])if(!env[name])throw new Error(`missing ${name}`)
 if(target.startsWith('darwin'))macSigningConfig(env)
 if(target.startsWith('windows')&&!/^[A-Fa-f0-9]{40}$/.test(env.WINDOWS_SIGN_CERT_SHA1))throw new Error('invalid Windows certificate SHA1')
 return keys
}
export function macSigningConfig(env){
 if(!env.APPLE_SIGNING_IDENTITY||env.APPLE_SIGNING_IDENTITY==='-')throw new Error('macOS certificate identity required; ad hoc signing is not a release signature')
 if(!/^[A-Fa-f0-9]{40}$/.test(env.APPLE_SIGNING_CERT_SHA1??''))throw new Error('invalid Apple certificate SHA1')
 return {minimumSystemVersion:'12.0',signingIdentity:env.APPLE_SIGNING_CERT_SHA1}
}
export function build(target,env=process.env){
 const keys=preflight(env,target),v=version(env.RELEASE_VERSION),root=process.cwd()
 const work=path.resolve('.build/native-release',target),out=path.join(work,'payload'),assets=path.resolve('.build/native-release/assets')
 fs.mkdirSync(work,{recursive:true});fs.mkdirSync(assets,{recursive:true})
 const go=env.NATIVE_REAL_GO??path.resolve('.build/toolchain/go/bin/go'),tool=env.MYGO_CLI??path.resolve('.build/tools/mygo'+(os.platform()==='win32'?'.exe':''))
 const toolVersion=spawnSync(go,['version'],{encoding:'utf8',env});if(toolVersion.status!==0||!toolVersion.stdout.includes('go1.27.1 '))throw new Error('Go 1.27.1 required')
 const module=spawnSync(go,['list','-m','-f','{{.Version}}','github.com/egoist/mygo'],{encoding:'utf8',env});if(module.status!==0||module.stdout.trim()!=='v0.6.2')throw new Error('mygo v0.6.2 required')
 const wrapper=path.join(work,'toolchain');fs.mkdirSync(wrapper,{recursive:true})
 run(go,['build','-o',path.join(wrapper,os.platform()==='win32'?'go.exe':'go'),'./scripts/native-release-go'],env)
 const config={...JSON.parse(fs.readFileSync('mygo.json','utf8')),main:root,icon:path.resolve('native/app-icon.png'),out,bindings:path.resolve('.build/native-bindings.ts'),version:v,linux:{webview:false},urlSchemes:['xiaobaiswitch'],updates:{publicKey:env.MYGO_UPDATER_PUBLIC_KEY,github:'Licoy/xiaobai-switch',deltas:0},macos:target.startsWith('darwin')?macSigningConfig(env):{minimumSystemVersion:'12.0'}}
 if(config.i18n?.dir)config.i18n={...config.i18n,dir:path.resolve(config.i18n.dir)}
 fs.writeFileSync(path.join(work,'mygo.json'),JSON.stringify(config,null,2))
 const buildEnv={...env,PATH:wrapper+path.delimiter+env.PATH,NATIVE_REAL_GO:go,RELEASE_VERSION:v}
 run(tool,['build','-platform',target.replace('-','/'),'-skip-build-command',work],buildEnv)
 const stage=path.join(out,target),files=fs.readdirSync(stage)
 const original=files.find(n=>n.endsWith('.tar.gz'))
 if(!original)throw new Error('MyGo skipped the signed update archive')
 const archiveName=assetName(v,target)
 fs.copyFileSync(path.join(stage,original),path.join(assets,archiveName))
 const packaging={work,stage,files,original,assets,archiveName,target,v,env}
 if(target.startsWith('windows'))packageWindows(packaging)
 if(target.startsWith('darwin'))packageMac(packaging)
 if(target.startsWith('linux')){const script=path.join(stage,'install.sh');if(!fs.existsSync(script))throw new Error('missing Linux install.sh');fs.copyFileSync(script,path.join(assets,'install.sh'))}
 const data=fs.readFileSync(path.join(assets,archiveName)),baseURL=`https://github.com/Licoy/xiaobai-switch/releases/download/v${v}`
 fs.writeFileSync(path.join(assets,`update-${target}.json`),JSON.stringify(nativeManifest({v,target,data,baseURL,keys,notes:releaseNotes(env.RELEASE_NOTES)}),null,2)+'\n')
 if(!target.startsWith('linux'))run(process.execPath,[path.resolve('node_modules/@tauri-apps/cli/tauri.js'),'signer','sign','--private-key',env.TAURI_SIGNING_PRIVATE_KEY,'--password',env.TAURI_SIGNING_PRIVATE_KEY_PASSWORD,path.join(assets,assetName(v,target,target.startsWith('darwin')?'app.tar.gz':'setup.exe'))],env)

 // Archive listings must be relative and must never escape the installation root.
 const listing=spawnSync('tar',['-tzf',path.join(assets,archiveName)],{encoding:'utf8'})
 if(listing.status!==0)throw new Error('cannot inspect update archive')
 for(const entry of listing.stdout.trim().split('\n'))safeEntry(entry)
 return assets
}
if(process.argv[1]===fileURLToPath(import.meta.url))build(process.argv[2]??`${os.platform()}-${os.arch()}`)
