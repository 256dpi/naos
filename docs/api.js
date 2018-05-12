let fs = require('fs');
let xml = require('xml2js');

let fileNames = fs.readdirSync("./doxygen/xml");

let enums = [];
let structs = [];
let functions = [];

fileNames.forEach(function(fileName) {
  if(!fileName.match(".*.xml")) {
    return;
  }

  console.log("parsing " + fileName);

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
        s.Fields.push({
          Type: md['type'][0]+md['argsstring'][0],
          Name: md['name'][0],
          Description: md['detaileddescription'][0]['para'][0],
        });
      });

      structs.push(s);
    }

    if(kind === 'file') {
      cd['sectiondef'].forEach((sd) => {
        sd['memberdef'].forEach((md) => {
          if(md['$']['kind']  === 'enum') {
            enums.push({
              Name: md['name'][0],
              Values: md['enumvalue'].map((ev) => {
                return {
                  Name: ev['name'][0],
                  Description: ev['detaileddescription'][0]['para'][0],
                };
              })
            });
          }

          if(md['$']['kind'] === 'function') {
            let f = {
              Name: md['name'][0],
              Type: md['type'][0],
              Description: "",
              Note: null,
              Params: [],
              Returns: null
            };

            md['detaileddescription'][0]['para'].forEach((p) => {
              if(typeof p === 'string') {
                f.Description = p;
                return;
              }

              if (p['parameterlist']) {
                f.params = (p['parameterlist'][0]['parameteritem'].map((pi) => {
                  return {
                    Name: pi['parameternamelist'][0]['parametername'][0],
                    Description: pi['parameterdescription'][0]['para'][0]
                  };
                }));
              }

              if (p['simplesect']) {
                if(p['simplesect'][0]['$']['kind'] === 'note') {
                  f.Note = p['simplesect'][0]['para'][0];
                } else if(p['simplesect'][0]['$']['kind'] === 'return') {
                  f.Returns = p['simplesect'][0]['para'][0];
                }
              }
            });

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

console.log("done!");
