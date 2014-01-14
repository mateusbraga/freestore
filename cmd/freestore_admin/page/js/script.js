function dataCtrl($scope, $timeout, Data) {
    $scope.data = [];

    (function tick() {
        $scope.data = Data.query(function(){
            $timeout(tick, 1000);
        });
    })();
};
