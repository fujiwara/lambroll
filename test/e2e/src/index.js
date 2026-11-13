exports.handler = async function (event) {
    return { message: process.env.MESSAGE, event: event }
}
