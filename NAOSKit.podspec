Pod::Spec.new do |s|
  s.name = 'NAOSKit'
  s.version = '0.0.0'
  s.summary = 'The Networked Artifacts Operating System.'
  s.homepage = 'https://github.com/256dpi/naos'
  s.license = 'MIT'
  s.author = { "Joël Gähwiler" => "joel.gaehwiler@gmail.com" }
  s.source = { git: 'https://github.com/256dpi/naos.git', tag: 'master' }
  s.source_files = 'swift/NAOSKit/*.swift'
  s.osx.deployment_target = '11.0'
  s.ios.deployment_target = '14.0'
end
