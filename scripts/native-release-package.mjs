import fs from 'node:fs'
import path from 'node:path'
import {spawnSync} from 'node:child_process'
import {redact} from './native-release-log.mjs'
import {signWindowsFile} from './native-release-sign.mjs'
import {assetName,safeEntry} from './native-release-lib.mjs'
export function run(command,args,env=process.env){const r=spawnSync(command,args,{stdio:'inherit',env});if(r.status!==0)throw new Error(`${path.basename(command)} failed (${r.status??r.error?.code})`)}
// Git Bash can put GNU tar first; it treats Windows drive letters as remote hosts.
export function archiveTool(env=process.env,platform=process.platform){
  if(platform!=='win32')return 'tar'
  const systemRoot=Object.entries(env).find(([name])=>name.toLowerCase()==='systemroot')?.[1]
  if(!path.win32.isAbsolute(systemRoot??''))throw new Error('absolute SystemRoot required for Windows archive tool')
  return path.win32.join(systemRoot,'System32','tar.exe')
}
export function inspectArchive(file,env=process.env){
  const tool=archiveTool(env),result=spawnSync(tool,['-tzf',file],{encoding:'utf8',env})
  if(result.status!==0)throw new Error(redact(`cannot inspect archive with ${tool} (${result.status??result.error?.code}): ${result.stderr??''} ${result.error?.message??''}`,env))
  const entries=result.stdout.trim().split(/\r?\n/)
  for(const entry of entries)safeEntry(entry)
  return entries
}
export function packageWindows({work,stage,files,original,assets,archiveName,target,v,env}){
  const payload=path.join(work,'installer-payload');fs.mkdirSync(payload,{recursive:true})
  const archivePath=path.join(stage,original)
  inspectArchive(archivePath,env)
  run(archiveTool(env),['-xzf',archivePath,'-C',payload],env)
  const exe=path.join(payload,'XiaoBaiSwitch.exe');if(!fs.existsSync(exe))throw new Error('missing Windows payload')
  const signFile=f=>signWindowsFile(f,env)
  signFile(exe)
  const installer=path.join(assets,assetName(v,target,'setup.exe'))
  run(env.MAKENSIS??'makensis',[`/DVERSION=${v}`,`/DFILEVERSION=${v.split('-')[0]}.0`,`/DPAYLOAD=${payload}`,`/DOUTPUT=${installer}`,'/DUNINSTALLSIGN','scripts/native-transition-installer.nsi'],{...env,NATIVE_SIGN_SCRIPT:path.resolve('scripts/native-release-sign.mjs')})
  if(!fs.existsSync(installer)||!fs.statSync(installer).size)throw new Error('NSIS skipped the installer')
  signFile(installer)
  run('powershell',['-NoProfile','-Command',`Compress-Archive -Path '${payload.replaceAll("'","''")}\\*' -DestinationPath '${path.join(assets,assetName(v,target,'zip')).replaceAll("'","''")}' -Force`],env)
  // Repack after Authenticode, so the native updater installs the same signed executable.
  run(archiveTool(env),['-czf',path.join(assets,archiveName),'-C',payload,...fs.readdirSync(payload)],env)

}
export function packageMac({stage,files,assets,archiveName,target,v,env}){
  const app=files.find(n=>n.endsWith('.app')),dmg=files.find(n=>n.endsWith('.dmg'))
  if(!app||!dmg)throw new Error('missing signed macOS app or disk image')
  if(!/^[A-Fa-f0-9]{40}$/.test(env.APPLE_SIGNING_CERT_SHA1??''))throw new Error('Apple certificate SHA1 required')
  const requirement=`=certificate leaf = H"${env.APPLE_SIGNING_CERT_SHA1}"`
  run('codesign',['--verify','--deep','--strict','-R',requirement,path.join(stage,app)],env)
  run('codesign',['--verify','--strict','-R',requirement,path.join(stage,dmg)],env)
  // Self-signed DMG containers add a Gatekeeper rejection; retain the signed app inside.
  const unsigned=path.join(assets,assetName(v,target,'dmg'))
  run('hdiutil',['convert',path.join(stage,dmg),'-format','ULMO','-ov','-o',unsigned],env)
  run('hdiutil',['verify',unsigned],env)
  const signature=spawnSync('codesign',['--display',unsigned],{encoding:'utf8',env:{...env,LC_ALL:'C'}})
  assertUnsignedDMG(signature)
  run(archiveTool(env),['-czf',path.join(assets,assetName(v,target,'app.tar.gz')),'-C',stage,app],{...env,COPYFILE_DISABLE:'1'})
  fs.copyFileSync(path.join(assets,assetName(v,target,'app.tar.gz')),path.join(assets,archiveName))
}

export function assertUnsignedDMG(result){
 if(result.error||result.status!==1||!`${result.stdout}${result.stderr}`.includes('code object is not signed at all'))throw new Error('rebuilt DMG is not confirmed unsigned')
}
