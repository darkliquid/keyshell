pragma Singleton
import QtQuick
import Quickshell
import qs.components
import "calc.js" as Calc

Singleton {
    id: calcBackend

    readonly property string id: "calc"
    readonly property string name: "Calculator"
    readonly property string badge: "CALC"
    readonly property string prefix: "="
    readonly property bool searchable: false
    readonly property bool trackable: false
    readonly property Component cardDelegate: CalcCardDelegate {}

    signal resultsReady(string query, var items)

    function search(q, limit) {
        const expression = (q || "").trim();
        if (expression.length === 0 || expression.length > Config.maxExpressionLength)
            return [];

        const value = Calc.evaluate(expression);
        if (value === null)
            return [];

        const result = Calc.format(value, Config.calcMaxDecimals);
        if (result.length === 0)
            return [];

        return [
            {
                "id": "calc:" + expression,
                "title": result,
                "subtitle": expression,
                "hint": "[ENTER] COPY RESULT",
                "icon": "",
                "previewSource": null,
                "badge": "CALC",
                "delegate": calcBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Copy result",
                        "execute": function () {
                            Quickshell.execDetached(["wl-copy", result]);
                        }
                    }
                ]
            }
        ];
    }

    function refresh() {
    }
}