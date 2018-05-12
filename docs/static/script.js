/* Post process API documentation and link types in definitions */

var types = document.getElementsByClassName('js-type');
var list = document.getElementsByTagName('code');

for (var i=0; i<list.length; i++) {
  for (var j=0; j<types.length; j++) {
    var el = list[i];
    var t = types[j].innerText;

    if(el.innerText.match(t)) {
      console.log(el, t);
      console.log(el.innerHTML.replace(t, '<a href="#' + t + '">' + t + '</a>'));
      el.innerHTML = el.innerHTML.replace(t, '<a href="#' + t + '">' + t + '</a>');
    }
  }
}
