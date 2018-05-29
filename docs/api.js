let fs = require('fs');
let xml = require('xml2js');

let fileNames = fs.readdirSync("./doxygen/xml");

let enums = [];
let structs = [];
let functions = [];

function parseType(type) {
  if(type['ref']) {
    let suffix = '';
    if(type['_']) {
      suffix = type['_'];
    }

    return type['ref'][0]['_'] + suffix;
  }

  return type;
}

function parseDescription(list) {
  let d = {};

  list.forEach((p) => {
    if(typeof p === 'string') {
      d.Description = p;
      return;
    }

    if (p['parameterlist']) {
      d.Params = (p['parameterlist'][0]['parameteritem'].map((pi) => {
        return {
          Name: pi['parameternamelist'][0]['parametername'][0],
          Description: pi['parameterdescription'][0]['para'][0]
        };
      }));
    }

    if (p['simplesect']) {
      p['simplesect'].forEach((sc) => {
        if(sc['$']['kind'] === 'note') {
          d.Note = sc['para'][0];
        } else if(sc['$']['kind'] === 'return') {
          d.Returns = sc['para'][0];
        }
      });
    }
  });

  return d;
}

function parseParams(list) {
  return list.map((p) => {
    let t = parseType(p['type'][0]);

    return {
      Type: t,
      Name: p['declname'] ? p['declname'][0] : t
    }
  });
}

function parseArgString(str) {
  if(!str) {
    return [];
  }

  let p = str.slice(2, -1).split(', ');
  if (!p[0]) {
    return [];
  }

  return p.map((p) => {
    let i = p.lastIndexOf('*');
    if(i >= 0) {
      return {
        Type: p.slice(0, i+1),
        Name: p.slice(i+1)
      };
    }

    i = p.lastIndexOf(' ');
    return {
      Type: p.slice(0, i),
      Name: p.slice(i+1)
    };
  });
}

fileNames.forEach(function(fileName) {
  if(!fileName.match(".*.xml")) {
    return;
  }

  let content = fs.readFileSync("./doxygen/xml/" + fileName, 'utf-8');

  xml.parseString(content, function (err, result) {
    if(!result['doxygen']) {
      return;
    }

    let cd = result['doxygen']['compounddef'][0];

    let kind = cd['$']['kind'];
    if (kind !== 'file' && kind !== 'struct') {
      return;
    }

    if(kind === 'struct') {
      let s = {
        Name: cd['compoundname'][0],
        Description: cd['detaileddescription'][0]['para'][0],
        Fields: []
      };

      cd['sectiondef'][0]['memberdef'].forEach((md) => {
        let d = parseDescription(md['detaileddescription'][0]['para']);
        let p = parseArgString(md['argsstring'][0]);

        p.forEach((p1)=>{
          let p2 = d.Params.find((p2)=> p2.Name === p1.Name);
          p2['Type'] = p1.Type;
        });

        let a = md['argsstring'][0];
        let t = parseType(md['type'][0]);
        let n = md['name'][0];

        if(a) {
          s.Fields.push({
            Kind: 'function',
            Definition: t + n + a,
            Type: t.slice(0, t.indexOf('(')),
            Name: n,
            Description: d.Description || '',
            Params: d.Params || [],
            Returns: d.Returns || null
          });
        } else {
          s.Fields.push({
            Kind: 'variable',
            Definition: `${t} ${n}`.replace('* ', '*'),
            Type: t,
            Name: n,
            Description: d.Description || ''
          });
        }
      });

      structs.push(s);
    }

    if(kind === 'file') {
      cd['sectiondef'].forEach((sd) => {
        sd['memberdef'].forEach((md) => {
          if(md['$']['kind']  === 'enum') {
            enums.push({
              Name: md['name'][0],
              Description: md['detaileddescription'][0]['para'][0],
              Values: md['enumvalue'].map((ev) => {
                return {
                  Name: ev['name'][0],
                  Description: ev['detaileddescription'][0]['para'][0],
                };
              })
            });
          }

          if(md['$']['kind'] === 'function') {
            let d = parseDescription(md['detaileddescription'][0]['para']);
            let p = parseParams(md['param'] || []);

            p.forEach((p1)=>{
              let p2 = d.Params.find((p2)=> p2.Name === p1.Name);
              p2['Type'] = p1.Type;
            });

            let t = parseType(md['type'][0]);
            let n = md['name'][0];
            let a = md['argsstring'][0];

            let f = {
              Definition: `${t} ${n}${a}`.replace('...', ' ...').replace('* ', '*'),
              Name: n,
              Type: t,
              Description: d.Description || '',
              Note: d.Note || null,
              Params: d.Params || [],
              Returns: d.Returns || null
            };

            functions.push(f);
          }
        });
      });
    }
  });
});

fs.writeFileSync('./data/api.json', JSON.stringify({
  Enums: enums,
  Functions: functions,
  Structs: structs
}, null, '  '));
