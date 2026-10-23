import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {generateKeyPairSync,createHash,sign} from 'node:crypto'
import {assemble} from './native-transition.mjs'
import {preflight} from './native-release.mjs'
import {targets,assetName,signingKeys,nativeManifest} from './native-release-lib.mjs'
test('release missing credentials fails before modifying any files',()=>{assert.throws(()=>preflight({RELEASE_VERSION:'1.0.0'},'darwin-arm64'));assert.throws(()=>preflight({RELEASE_VERSION:'0.0.12'},'windows-amd64'))})
test('publication requires six signed archives plus immutable bridge',()=>{
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),'xiaobai-release-fixture-'))
 try{
  const pair=generateKeyPairSync('ed25519'),pub=pair.publicKey.export({type:'spki',format:'der'}).subarray(-32),seed=pair.privateKey.export({type:'pkcs8',format:'der'}).subarray(-32),keys=signingKeys(pub.toString('base64'),Buffer.concat([seed,pub]).toString('base64'))
  const baseURL='https://github.com/Licoy/xiaobai-switch/releases/download/v1.0.0',data=Buffer.from('fixture'),platforms={}
  const id=Buffer.alloc(8),sig=sign(null,createHash('blake2b512').update(data).digest(),pair.privateKey),packet=Buffer.concat([Buffer.from('ED'),id,sig]),comment='fixture'
  const signature=Buffer.from(`untrusted comment: test\n${packet.toString('base64')}\ntrusted comment: ${comment}\n${sign(null,Buffer.concat([sig,Buffer.from(comment)]),pair.privateKey).toString('base64')}\n`).toString('base64')
  const tauriPublicKey=Buffer.from(`untrusted comment: public key\n${Buffer.concat([Buffer.from('Ed'),id,pub]).toString('base64')}\n`).toString('base64')
  for(const target of targets){
   fs.writeFileSync(path.join(directory,assetName('1.0.0',target)),data)
   fs.writeFileSync(path.join(directory,`update-${target}.json`),JSON.stringify(nativeManifest({v:'1.0.0',target,data,baseURL,keys})))
   if(target.startsWith('linux')){fs.writeFileSync(path.join(directory,'install.sh'),'#!/bin/sh\n');continue}
   fs.writeFileSync(path.join(directory,assetName('1.0.0',target,target.startsWith('darwin')?'dmg':'zip')),data)
   const name=assetName('1.0.0',target,target.startsWith('darwin')?'app.tar.gz':'setup.exe');fs.writeFileSync(path.join(directory,name),data);fs.writeFileSync(path.join(directory,name+'.sig'),signature)
   platforms[target.replace('amd64','x86_64').replace('arm64','aarch64')]={signature,url:'https://github.com/Licoy/xiaobai-switch/releases/download/v0.0.12/legacy'}
  }
  for(const key of Object.keys(platforms))platforms[key+(key.startsWith('darwin')?'-app':'-nsis')]=platforms[key]
  platforms['windows-x86_64-msi']=platforms['windows-x86_64']
  const bridgeFile=path.join(directory,'bridge.json');fs.writeFileSync(bridgeFile,JSON.stringify({version:'0.0.12',platforms}))
  assert.equal(assemble({directory,v:'1.0.0',baseURL,bridgeFile,publicKey:pub.toString('base64'),tauriPublicKey}).version,'1.0.0')
  assert.equal(JSON.parse(fs.readFileSync(path.join(directory,'latest.json'))).version,'0.0.12')
  fs.writeFileSync(path.join(directory,assetName('1.0.0','linux-arm64')),Buffer.from('tampered'))
  assert.throws(()=>assemble({directory,v:'1.0.0',baseURL,bridgeFile,publicKey:pub.toString('base64'),tauriPublicKey}))
 }finally{fs.rmSync(directory,{recursive:true,force:true})}
})
test('NSIS stages and verifies native before MSI uninstall, preserves rollback',()=>{
 const source=fs.readFileSync('scripts/native-transition-installer.nsi','utf8').replace(/\r\n/g,'\n')
 assert(source.indexOf('nsExec::ExecToStack')<source.indexOf('/x $MsiProduct'))
 for(const root of ['HKCU','HKLM'])for(const view of [32,64])assert(source.includes(`FindMsi ${root} ${view}`))
 assert(source.includes('Rename "$INSTDIR.previous" "$INSTDIR"'))
 for(const flag of ['/UPDATE','/ARGS','/R','/P'])assert(source.includes(`"${flag}"`))
 assert(source.includes('StrCpy $INSTDIR "$LOCALAPPDATA\\${PRODUCT}"'))
 assert(!source.includes('StrCpy $INSTDIR $MsiLocation'))
 assert(source.includes('previous files remain for recovery'))
 assert(source.includes('!define MANU "Software\\github\\${PRODUCT}"'))
 assert(source.includes('${AndIf} $4 == "github"'))
 assert(source.includes('Function .onInstFailed\n  SetErrorLevel 1'))
})
