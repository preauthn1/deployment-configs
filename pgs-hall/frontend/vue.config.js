
var path = require('path');

module.exports = {
    publicPath: './',   
    assetsDir: 'static',
    outputDir: '../views',
    chainWebpack: config => {
        config
            .plugin('html')
            .tap(args => {
                args[0].title = "";
                return args;
            });

        //bn.js這個套件被多個套件相依，共用此可以減少打包的容量25%
        config.resolve.alias.set('bn.js', path.resolve('node_modules/bn.js/lib/bn.js')); 
    }
}