$(function () {
  $.fn.parseForm = function () {
    var itemForm;
    if ($(this).prop("nodeName").toLowerCase() == "form") {
      itemForm = $(this);
    } else {
      itemForm = $(this).parents("form");
    }

    var field = {},
      fieldElem = itemForm.find("input,select,textarea"); // 获取所有表单域

    $.each(fieldElem, function (_, item) {
      var init_name; // 初始 name

      item.name = (item.name || "").replace(/^\s*|\s*&/, "");
      if (!item.name) return;

      if (/^checkbox|radio$/.test(item.type) && !item.checked) return; // 复选框和单选框未选中，不记录字段

      // 用于支持数组 name
      if (/^.*\[\]$/.test(item.name)) {
        var key = item.name.match(/^(.*)\[\]$/g)[0];
        if (!field[key]) {
          field[key] = [];
        }
        field[key].push(item.value);
      } else if (
        item.name.indexOf("[") !== -1 ||
        item.name.indexOf("[") !== -1
      ) {
        init_name = item.name.replace("[", ".").replace("]", "");
        init_name = init_name.split(".");
        if (!field[init_name[0]]) {
          field[init_name[0]] = {};
        }
        if (
          init_name[1].indexOf("[") !== -1 ||
          init_name[1].indexOf("[") !== -1
        ) {
          init_name2 = init_name[1].replace("[", ".").replace("]", "");
          init_name2 = init_name2.split(".");
          if (!field[init_name[0]][init_name2[0]]) {
            field[init_name[0]][init_name2[0]] = {};
          }
          field[init_name[0]][init_name2[0]][init_name2[1]] = item.value;
        } else {
          field[init_name[0]][init_name[1]] = item.value;
        }
      } else {
        field[init_name || item.name] = item.value;
      }
    });

    return field;
  };
  /**
   *
   * @param {data: {}, action: string, callback: ():void => {}} obj
   */
  $.fn.jsonPost = function (obj = {}) {
    let action = $(this).parents("form").attr("action") || "";
    if (obj.action) {
      action = obj.action;
    }
    $.ajax({
      url: action,
      contentType: "application/json;charset=utf-8",
      data: JSON.stringify(obj.data),
      type: "post",
      dataType: "json",
      async: false,
      success: function (res) {
        if (obj.callback) {
          obj.callback(res);
        } else {
          $(this).showToast({ message: res.msg }, function () {
            if (res.code === 0) {
              window.location.reload();
            }
          });
        }
      },
    }).fail(function (err) {
      if (obj.fail) {
        obj.fail(err);
      }
      $(this).showToast({ message: "Submission error" });
    });
  };

  $.fn.showToast = function (options, callback = null) {
    if (!options.delay) {
      options.delay = 2000;
    }
    $("body")
      .append(`<div class="toast-container position-fixed end-0 p-3" style="top: 100px;">
    <div id="defaultToast" class="toast border-0 bg-dark text-white" role="alert" aria-live="assertive" aria-atomic="true">
      <div class="d-flex">
        <div class="toast-body">
          ${options.message}
        </div>
        <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
      </div>
    </div>
  </div>`);
    const toast = new bootstrap.Toast($("#defaultToast")[0], {
      delay: options.delay,
    });
    toast.show();
    $("#defaultToast").on("hidden.bs.toast", function (e) {
      $("#defaultToast").remove();
    });
    setTimeout(function () {
      toast.hide();
      if (callback && typeof callback === "function") {
        callback();
      }
    }, options.delay);
  };

  // Add sticky header effect
  const $header = $("header");
  $(window).on("scroll", function () {
    if ($(window).scrollTop() > 50) {
      $header.addClass("shadow-sm");
    } else {
      $header.removeClass("shadow-sm");
    }
  });

  // Smooth scroll for anchor links
  $('a[href^="#"]').on("click", function (e) {
    const href = $(this).attr("href");
    if (href !== "#" && $(href).length) {
      e.preventDefault();
      $("html, body").animate(
        {
          scrollTop: $(href).offset().top,
        },
        500,
      );
    }
  });

  // Form validation example
  $(".needs-validation").on("submit", function (event) {
    const form = this;
    if (!form.checkValidity()) {
      event.preventDefault();
      event.stopPropagation();
    }
    $(form).addClass("was-validated");
  });

  var referer = document.referrer;
  if (
    referer.indexOf("/login") > -1 ||
    referer.indexOf("/register") > -1 ||
    referer.indexOf("/logout") > -1
  ) {
    referer = "/";
  }
  // Login success toast logic
  var userInfo = JSON.parse(localStorage.getItem("user")) || {};
  $(".header-account").prop(
    "href",
    userInfo.id ? "/account/index" : "/register",
  );
  $(".header-account-mobile")
    .prop("href", userInfo.id ? "/account/index" : "/register")
    .text(userInfo.id ? "Account" : "Login / Register");
  $.get(
    "/api/user/detail",
    {},
    function (res) {
      if (res.data) {
        if (res.data.id != userInfo.id) {
          localStorage.setItem("user", JSON.stringify(res.data));
        }
        userInfo = res.data;
      } else {
        userInfo = {};
      }
      $(".header-account").prop(
        "href",
        userInfo.id ? "/account/index" : "/login",
      );
      $(".header-account-mobile")
        .prop("href", userInfo.id ? "/account/index" : "/register")
        .text(userInfo.id ? "Account" : "Login / Register");
    },
    "json",
  );
  $(document).on("click", "#logout", function (e) {
    e.preventDefault();
    localStorage.removeItem("user");
    userInfo = {};
    $(".header-account").prop("href", "/register");
    $(".header-account-mobile")
      .prop("href", "/register")
      .text("Login / Register");
    window.location.href = "/logout";
  });
  $("#loginForm").on("submit", function (e) {
    e.preventDefault();
    let data = $(this).parseForm();
    $(this).jsonPost({
      data: data,
      action: "/api/login",
      callback: function (res) {
        $(this).showToast(
          { message: res.msg || "Login Sucessfully.", delay: 1500 },
          function () {
            if (res.code === 0) {
              window.location.href = referer;
            }
          },
        );
      },
    });
    return false;
  });
  $("#registerForm").on("submit", function (e) {
    e.preventDefault();
    let data = $(this).parseForm();
    data.real_name = data.first_name + " " + data.last_name;
    data.user_name =
      data.real_name.replace(" ", "_") + "_" + data.email.split("@")[0];
    $(this).jsonPost({
      data: data,
      action: "/api/register",
      callback: function (res) {
        $(this).showToast(
          { message: res.msg || "Register Sucessfully.", delay: 1500 },
          function () {
            if (res.code === 0) {
              window.location.href = referer;
            }
          },
        );
      },
    });
    return false;
  });
  // google login
  $(document).on("click", ".google-login-btn", function () {
    $.get("/api/google/url").then((res) => {
      if (res.data) {
        sessionStorage.setItem("referer", referer);
        sessionStorage.setItem("state", res.data.state);
        window.location.href = res.data.url;
      } else {
        $(this).showToast({ message: res.msg });
      }
    }, "json");
  });
  // 检查路由中有 code 参数，则请求后端检查
  if (
    window.location.href.indexOf("/login") > -1 &&
    window.location.href.indexOf("code") > -1 &&
    window.location.href.indexOf("state") != -1
  ) {
    // 获取 参数 code
    var urlParams = new URLSearchParams(window.location.search);
    var code = urlParams.get("code");
    var state = urlParams.get("state");
    $.ajax({
      type: "POST",
      url: "/api/login",
      data: JSON.stringify({
        platform: "google",
        state: state,
        code: code,
      }),
      contentType: "application/json",
      dataType: "json",
      success: function (res) {
        referer = sessionStorage.getItem("referer") || "/";
        $(this).showToast(
          { message: res.msg || "Login Sucessfully.", delay: 1500 },
          function () {
            if (res.code === 0) {
              window.location.href = referer;
            }
          },
        );
      },
    });
  }
  // forgot password
  $(document).on("submit", "#forgotPasswordForm", function (e) {
    let loginEmail = $("#loginEmail").val();
    $.ajax({
      type: "POST",
      url: "/api/verify/email",
      data: JSON.stringify({
        state: "reset",
        email: loginEmail,
        return: "json",
      }),
      contentType: "application/json",
      dataType: "json",
      success: function (res) {
        if (res.code === 0) {
          // 正确
          let link = "https://mail." + loginEmail.split("@")[1];
          $("#forgotPasswordForm")
            .html(`<div class="d-grid gap-3 mb-4"><p>Please <a href="${link}">log in to your email</a> to check the message and proceed to the next step.</p>
                <a href="/login" class="text-center text-dark small text-decoration-underline">Back to Login</a></div>`);
        } else {
          $(this).showToast({ message: "Email does not exist." });
        }
      },
    });
  });
  // reset password
  $(document).on("submit", "#resetPasswordForm", function (e) {
    let data = $(this).parseForm();
    $.ajax({
      type: "POST",
      url: "/api/password/reset",
      data: JSON.stringify(data),
      contentType: "application/json",
      dataType: "json",
      success: function (res) {
        if (res.code === 0) {
          // 正确
          $(this).showToast(
            {
              message:
                res.msg ||
                "Reset Password Sucessfully. Please use the new password to login.",
              delay: 3000,
            },
            function () {
              if (res.code === 0) {
                window.location.href = "/login";
              }
            },
          );
        } else {
          $(this).showToast({ message: res.msg });
        }
      },
    });
  });
  // end login

  // Initialize Fancybox
  if (typeof Fancybox !== "undefined") {
    Fancybox.bind("[data-fancybox]", {
      // Custom options
      Hash: false,
    });
  }

  // search
  let searchTimeout;
  $("#searchInput").on("input", function () {
    const query = $(this).val();
    clearTimeout(searchTimeout);
    if (!query.trim()) {
      return;
    }
    $("#search-more").removeClass("d-none").find("#search-arg").text(query);
    searchTimeout = setTimeout(function () {
      performSearch(query);
    }, 300);
  });
  function performSearch(query) {
    $.ajax({
      url: "/api/archive/list",
      method: "GET",
      data: { q: query, moduleId: 2, limit: 5 },
      success: function (resp) {
        displaySearchResults(resp.data || []);
      },
      error: function (xhr, status, error) {
        console.error("搜索请求失败:", error);
      },
    });
  }
  function displaySearchResults(results) {
    $(".search-results-preview .list-group").html(
      results.map(
        (
          item,
        ) => `<a href="${item.link}" class="list-group-item list-group-item-action border-0 px-0 d-flex align-items-center gap-3">
                  <div class="bg-light" style="width: 50px; height: 50px;">
                    <img src="${item.thumb}" class="w-100 h-100 object-fit-cover" alt="${item.title}">
                  </div>
                  <span class="small fw-medium">${item.title}</span>
                </a>`,
      ),
    );
  }
  // search end

  if ($("#filter-sidebar").length > 0) {
    let baseUrl = window.location.href.split("?")[0];
    let queryString = new URLSearchParams(window.location.search);
    $("#filter-sidebar")
      .find(".form-check-input")
      .change(function () {
        if ($(this).parents("a").length) {
          window.location.href = $(this).parents("a").attr("href");
          return;
        }
        let checked = $(this).is(":checked");
        if (checked) {
          queryString.append($(this).attr("name"), $(this).val());
        } else {
          queryString.delete($(this).attr("name"), $(this).val());
        }
        window.location.href = baseUrl + "?" + queryString.toString();
      });
    // price range
    function updatePriceRange() {
      let price_from = $('input[name="price_from"]').val();
      let price_to = $('input[name="price_to"]').val();
      if (price_from > 0 || price_to > 0) {
        queryString.set("price", price_from + "-" + price_to);
      } else {
        queryString.delete("price");
      }
      window.location.href = baseUrl + "?" + queryString.toString();
    }
    $('input[name="price_from"], input[name="price_to"]').on(
      "blur",
      function () {
        updatePriceRange();
      },
    );
    // 添加Enter键支持
    $('input[name="price_from"], input[name="price_to"]').on(
      "keypress",
      function (e) {
        if (e.which === 13) {
          // 13是Enter键的键码
          updatePriceRange();
        }
      },
    );
    // order by
    $("#order-by").val(queryString.get("order"));
    $("#order-by").change(function () {
      queryString.set("order", $(this).val());
      window.location.href = baseUrl + "?" + queryString.toString();
    });
  }
  // guestbook
  $("#guestbookForm").on("submit", function (e) {
    e.preventDefault();
    let data = $(this).parseForm();
    console.log(data);
    $(this).jsonPost({
      data: data,
      action: "/api/guestbook.html",
      callback: function (res) {
        if (res.code === 0) {
          $("#guestbookForm")[0].reset();
          $(this).showToast({ message: res.msg, delay: 5000 });
        } else {
          $(this).showToast({ message: res.msg });
        }
      },
    });
    return false;
  });
  // cart
  function renderCart(show = false) {
    $.get("/api/cart/list", {}, function (res) {
      const cartItemCount = res.data?.length || 0;
      $(".cart-total-items").text(cartItemCount);
      let totalPrice = 0;
      for (let i = 0; i < res.data?.length; i++) {
        totalPrice += res.data[i].price * res.data[i].quantity;
      }
      let subTotal = totalPrice;
      $('.cart-sub-amount').text(
        "$" + (subTotal > 0 ? (subTotal / 100).toFixed(2) : 0),
      );
      $(".cart-amount").text(
        "$" + (totalPrice > 0 ? (totalPrice / 100).toFixed(2) : 0),
      );
      // render - popup cart
      const goodsList = (res.data || []).map((item) => {
        if (item.archive?.price == item.origin_price) {
          item.origin_price = null;
        }
        if ((item.origin_amount = item.amount)) {
          item.origin_amount = null;
        }
        if (item.order_extra) {
          let orderExtras = [];
          item.order_fields.forEach(function (field) {
            let tpl = `<div class='field-item'><span class="label">${field.name}: </span>`;
            if (field.type == "texts") {
              tpl += '<div class="item-value">';
              for (let k in item.order_extra[field.field_name]) {
                let val = item.order_extra[field.field_name][k];
                tpl += `<div class="value-item"><span class="key">${val.key}: </span><span class="value">${val.value}</span></div>`;
              }
              tpl += "</div>";
            } else {
              tpl += `<span class="item-value">${item.order_extra[field.field_name]}</span>`;
            }
            tpl += "</div>";
            orderExtras.push(tpl);
          });
          item.order_extras = orderExtras.join("");
        }
        return item;
      });
      $("#popup-cart-items").html(
        goodsList.map(
          (
            item,
          ) => `<a href="${item.archive?.link || ''}" class="d-flex gap-3 mb-4 text-decoration-none text-dark">
            <div class="bg-light" style="width: 70px; height: 70px;">
              <img src="${item.archive?.thumb || ''}" class="w-100 h-100 object-fit-cover" alt="${item.archive?.title || ''}">
            </div>
            <div>
              <h6 class="small fw-bold mb-1">${item.archive?.title || ''}</h6>
              <p class="small text-muted mb-0">${item.sku?.title || ''} x ${item.quantity}</p>
            </div>
          </a>`,
        ),
      );
      if ($("#cart-list").length > 0) {
        if (cartItemCount > 0) {
          $("#cart-list").html(
            goodsList.map(
              (item) => `<tr>
                <td class="ps-0 py-4">
                  <a class="d-flex text-decoration-none text-dark" href="${item.archive?.link || ''}">
                    <div class="bg-light me-4" style="width: 120px; aspect-ratio: 1/1;">
                      <img src="${item.archive?.thumb || ''}" class="img-fluid object-fit-cover w-100 h-100" alt="${item.archive?.title || ''}">
                    </div>
                    <div>
                      <h6 class="fw-bold mb-1">${item.archive?.title || ''}</h6>
                      <p class="small text-muted mb-2">${item.sku_values?.map((val) => val.title).join(", ") || ''} ${item.order_extras || ''}</p>
                      <p class="small fw-bold mb-0">${item.price < item.origin_price ? '<del class="text-muted">$' + (item.origin_price / 100).toFixed(2) + '</del>' + ' ' : ''} $${(item.price / 100).toFixed(2)}</p>
                    </div>
                  </a>
                </td>
                <td class="text-center py-4" data-id="${item.id}">
                  <div class="input-group input-group-sm border border-dark mx-auto" style="width: 100px;">
                    <button class="btn btn-link text-dark p-0 border-0 text-decoration-none flex-grow-1 cart-quantity-reduce" type="button">
                      <i class="bi bi-dash"></i>
                    </button>
                    <input type="number" class="form-control border-0 text-center p-0 flex-grow-1 bg-transparent cart-quantity-input" value="${item.quantity}" min="1" max="${item.sku?.stock || item.archive?.stock || 0}" id="cart-quantity-${item.id}" />
                    <button class="btn btn-link text-dark p-0 border-0 text-decoration-none flex-grow-1 cart-quantity-increase" type="button">
                      <i class="bi bi-plus"></i>
                    </button>
                  </div>
                  <button class="btn btn-link text-muted small p-0 mt-2 text-decoration-none cart-remove">Remove</button>
                </td>
                <td class="text-end pe-0 py-4">
                  <span class="fw-bold">$${((item.price * item.quantity) / 100).toFixed(2)}</span>
                </td>
              </tr>`,
            ),
          );
        } else {
          $("#cart-list").html(
            `<tr><td colspan="3" class="text-center" style="padding: 180px 10px;">Your Cart is Empty!</td></tr>`,
          );
        }
      }
      if (show) {
        showCart();
      }
      // apply coupon to cart
      // 先看看sessionStorage是否有coupon_code
      let coupon_code = sessionStorage.getItem("coupon_code");
      if (coupon_code) {
        // 验证优惠券是否有效
        cartCheckCouponCode(coupon_code, goodsList, totalPrice);
      }
      $(document).on("click", "#cart-apply-coupon", function () {
        console.log("click apply coupon");
        let coupon_code = $('input[name="coupon_code"]').val();
        if (!coupon_code) {
          $(this).showToast({ message: "Please enter coupon code." });
          return;
        }
        cartCheckCouponCode(coupon_code, goodsList, totalPrice);
      });
    });
    return;
  }
  function cartCheckCouponCode(coupon_code, goodsList, totalPrice) {
    // 构建请求参数，需要携带购物车内的完整商品信息，
    let checkCouponData = {
      coupon_code: coupon_code,
      details: goodsList.map((item) => ({
        goods_id: item.archive_id,
        goods_sku_id: item.sku?.id || 0,
        title: item.archive?.title || '',
        sku_title: item.sku ? item.sku.title : "",
        sku_values: item.sku_values || [],
        price: item.price,
        origin_price: item.origin_price,
        quantity: item.quantity,
        thumb: item.archive?.thumb || '',
        amount: item.amount,
        wholesale: item.archive?.wholesale,
        origin_amount: item.origin_amount,
        order_extra: item.order_extra,
        order_extras: item.order_extras,
      })),
    }
    $(this).jsonPost({
      data: checkCouponData,
      action: "/api/coupon/check",
      callback: function (res) {
        if (res.code === 0) {
          // 有效
          totalPrice = totalPrice - res.data.coupon_money;
          $(".cart-amount").text(
            "$" + (totalPrice > 0 ? (totalPrice / 100).toFixed(2) : 0),
          );
          $('#cart-discount').text(
            "$" + (res.data.coupon_money / 100).toFixed(2),
          );
          $('input[name="coupon_code"]').val(coupon_code);
          sessionStorage.setItem("coupon_code", coupon_code);
        } else {
          // 无效
          $('#cart-discount').text(
            "$0.00",
          );
          $('input[name="coupon_code"]').val('');
          sessionStorage.removeItem("coupon_code");
          $(this).showToast({ message: res.msg });
        }
      },
    });
  }
  function showCart() {
    // Cart Notification Logic
    const $cartNotification = $("#cartNotification");

    $cartNotification.fadeIn();
    // Automatically hide after 5 seconds
    setTimeout(function () {
      $cartNotification.fadeOut();
    }, 5000);

    $("#closeCartNotification, #continueShopping").on("click", function () {
      $cartNotification.fadeOut();
    });
  }
  // update cart
  function updateCart(id, quantity) {
    $(this).jsonPost({
      data: {
        id: Number(id),
        quantity: Number(quantity) || 1,
      },
      action: "/api/cart/update",
      callback: function (res) {
        if (res.code === 0) {
          renderCart(false);
        } else {
          $(this).showToast({ message: res.msg });
        }
      },
    });
  }
  renderCart(false);
  // remove cart item
  $(document).on("click", ".cart-remove", function () {
    let cartId = $(this).closest("td").attr("data-id");
    $(this).jsonPost({
      action: "/api/cart/remove",
      data: { id: Number(cartId) },
      callback: function (res) {
        $(this).showToast(
          { message: res.msg || "Remove Sucessfully.", delay: 1500 },
          function () {
            renderCart(false);
          },
        );
      },
    });
  });
  // update cart quantity
  $(document).on("change", ".cart-quantity-input", function () {
    let cartId = $(this).closest("td").attr("data-id");
    let max = $(this).prop("max") || 1;
    let quantity = $(this).val();
    if (quantity > max) {
      quantity = max;
      $(this).val(quantity);
    }
    updateCart(cartId, quantity);
  });
  $(document).on("click", ".cart-quantity-reduce", function () {
    let cartId = $(this).closest("td").attr("data-id");
    let inputEl = $(this).closest("td").find("input");
    let max = inputEl.prop("max") || 1;
    let quantity = Number(inputEl.val());
    if (quantity > 1) {
      quantity = parseInt(quantity) - 1;
      inputEl.val(quantity);
      updateCart(cartId, quantity);
    }
  });
  $(document).on("click", ".cart-quantity-increase", function () {
    let cartId = $(this).closest("td").attr("data-id");
    let inputEl = $(this).closest("td").find("input");
    let max = Number(inputEl.prop("max")) || 1;
    let quantity = Number(inputEl.val());
    if (quantity < max) {
      quantity = quantity + 1;
      inputEl.val(quantity);
      updateCart(cartId, quantity);
    }
  });
  // product detail
  if ($("#productDetail").length > 0 && typeof productId !== "undefined") {
    // sku selector
    var defaultPrice = "";
    var sku = {};
    var selectedSkuId = Number(variantId) || 0; // 默认选中的sku id
    var selectedSku = {};
    var selectedValueItems = [];

    function updateStockStatus(stock) {
      if (stock < 1) {
        $("#product-availability").html(
          '<span class="badge bg-dark text-light border rounded-pill px-3 py-2 opacity-75 small">Sold Out</span>',
        );
        // btn
        $(".btn-add-to-cart,.btn-buy-now").addClass("disabled");
      } else {
        $("#product-availability").html(
          '<span class="badge bg-light text-dark border rounded-pill px-3 py-2 small">In Stock</span>',
        );
        // btn
        $(".btn-add-to-cart,.btn-buy-now").removeClass("disabled");
      }
      $('input[name="quantity"]').prop("max", stock);
      let quantity = $('input[name="quantity"]').val();
      if (quantity > stock) {
        quantity = stock;
        if (quantity < 1) {
          quantity = 1;
        }
        $('input[name="quantity"]').val(quantity);
      }
    }
    // 更新SKU选项可用性的函数
    function updateSkuOptionsAvailability() {
      // 先移除所有禁用状态
      $("label[data-value]").removeClass(
        "opacity-50 text-decoration-line-through",
      );

      // 获取当前所有已选中的SKU值，但排除正在选择的选项组
      let selectedValues = {};
      $(".sku-option").each(function () {
        let optionId = $(this).data("option");
        let selectedValue = $(this).find("label.active").data("value");
        if (selectedValue) {
          selectedValues[optionId] = parseInt(selectedValue);
        }
      });
      // 遍历每个选项组，检查其中的值是否与其他已选值形成有库存的组合
      $(".sku-option").each(function (index) {
        let currentOptionId = $(this).data("option");
        let isAttribute = $(this).data("is-attribute");
        // 对当前选项组中的每个值，检查是否存在一个完整组合且有库存
        $(this)
          .find("label[data-value]")
          .each(function () {
            const currentValueId = parseInt($(this).data("value"));
            // 创建一个临时的选中值对象，包含当前测试的值
            let tempSelectedValues = Object.assign({}, selectedValues);
            tempSelectedValues[currentOptionId] = currentValueId;
            // 检查是否存在一个包含这些值的SKU且有库存
            let hasAvailableSku = isAttribute == 'True' ? true : false;
            let hasShowSku = (index > 0 && isAttribute != 'True') ? false : true;

            for (let i = 0; i < sku.skus.length; i++) {
              const skuItem = sku.skus[i];

              // 检查此SKU是否有库存
              if (skuItem.stock > 0) {
                // 检查此SKU是否包含了当前选中的所有值
                let matchesAllSelected = true;

                for (let optionId in tempSelectedValues) {
                  let found = false;
                  let option = sku.options.find(opt => opt.id == optionId) || {};
                  // 属性值不在sku的values里，单独判断
                  if (option.is_attribute) {
                    found = true;
                  } else {
                    for (let j = 0; j < skuItem.values.length; j++) {
                      if (skuItem.values[j].id === tempSelectedValues[optionId]) {
                        found = true;
                        break;
                      }
                    }
                  }

                  if (!found) {
                    matchesAllSelected = false;
                    break;
                  }
                }

                if (matchesAllSelected) {
                  hasAvailableSku = true;
                  break;
                }
              }
            }
            // 检查此SKU是否有效（状态正常）
            for (let i = 0; i < sku.skus.length; i++) {
              const skuItem = sku.skus[i];
              if (skuItem.status !== 0) {
                // 检查此SKU是否包含了当前选中的所有值
                let matchesAllSelected = true;

                for (let optionId in tempSelectedValues) {
                  let found = false;
                  let option = sku.options.find(opt => opt.id == optionId) || {};
                  if (option.is_attribute) {
                    found = true;
                  } else {
                    for (let j = 0; j < skuItem.values.length; j++) {
                      if (skuItem.values[j].id === tempSelectedValues[optionId]) {
                        found = true;
                        break;
                      }
                    }
                  }

                  if (!found) {
                    matchesAllSelected = false;
                    break;
                  }
                }

                if (matchesAllSelected) {
                  hasShowSku = true;
                  break;
                }
              }
            }
            // 如果没有找到任何包含此值且有库存的完整SKU组合，则禁用该选项
            // 如果没有找到任何包含此值且有效（有库存且状态正常）的完整SKU组合，则隐藏该选项
            if (!hasShowSku) {
              $(this).addClass("d-none");
            } else {
              // 确保有效的选项是显示的
              $(this).removeClass("d-none");
            }
            if (!hasAvailableSku) {
              $(this).addClass("opacity-50 text-decoration-line-through");
            }
          });
      });
    }
    $.get(
      "/api/archive/sku",
      { id: productId },
      function (res) {
        if (res.code === 0) {
          sku = res.data;
          // default price
          if (sku.min_price != sku.max_price) {
            defaultPrice =
              sku.currency_symbol +
              (sku.min_price / 100).toFixed(2) +
              " - " +
              sku.currency_symbol +
              (sku.max_price / 100).toFixed(2);
          } else {
            defaultPrice = sku.currency_symbol + (sku.price / 100).toFixed(2);
          }
          $("#product-price").html(defaultPrice);
          if (sku.origin_price > 0 && sku.price < sku.original_price) {
            $("#origin-price").html(
              sku.currency_symbol + (sku.origin_price / 100).toFixed(2),
            );
          } else {
            $("#origin-price").hide();
          }
          // stock
          updateStockStatus(sku.stock);
          // 加价SKU模式 (option_type == 2)
          if (sku.option_type == 2 && sku.options && sku.options.length > 0) {
            // 设置默认选中的属性值（选择各属性的第一个默认值或第一个有库存的值）
            for (let i = 0; i < sku.options.length; i++) {
              let option = sku.options[i];
              let selectedValue = null;
              // 优先选择有库存的默认值
              // 选择默认值
              for (let j = 0; j < option.values.length; j++) {
                let val = option.values[j];
                if (val.is_default && (val.stock > 0 || val.is_attribute)) {
                  selectedValue = val;
                  break;
                }
              }
              if (!selectedValue) {
                // 选择默认值
                for (let j = 0; j < option.values.length; j++) {
                  let val = option.values[j];
                  if (option.is_attribute || val.stock > 0) {
                    selectedValue = val;
                    break;
                  }
                }
              }
              // 设置选中项
              if (selectedValue) {
                selectedValue.option_title = option.title;
                selectedValueItems.push(selectedValue);
                let valEl = $(`label[data-value="${selectedValue.id}"]`);
                valEl
                  .addClass("active")
                  .siblings()
                  .removeClass("active")
                  .find("input")
                  .prop("checked", false);
                valEl.find("input").prop("checked", true);
                valEl
                  .closest(".sku-option")
                  .find(".selectedValue")
                  .text(selectedValue.title);
              }
            }
            updatePriceAndStockForAddonMode();
            getSelectedValueItems();
            // 加价模式下的属性选择事件
            $(`label[data-value]`).click(function (e) {
              // 检查是否有库存
              if ($(this).hasClass("disabled-stock")) {
                e.preventDefault();
                return;
              }

              $(this)
                .addClass("active")
                .siblings()
                .removeClass("active")
                .find("input")
                .prop("checked", false);
              $(this).find("input").prop("checked", true);
              $(this)
                .closest(".sku-option")
                .find(".selectedValue")
                .text($(this).text());
              // 更新价格和库存
              updatePriceAndStockForAddonMode();
              getSelectedValueItems();
            });
          } else if (sku.skus.length > 0) {
            // 普通多规格模式
            // 组合SKU模式 (option_type == 1)
            // 设置属性的选项值
            for (let i = 0; i < sku.options.length; i++) {
              if (!sku.options[i].is_attribute) {
                continue;
              }
              let option = sku.options[i];
              let selectedValue = null;
              // 选择默认值
              for (let j = 0; j < option.values.length; j++) {
                let val = option.values[j];
                if (val.is_default) {
                  selectedValue = val;
                  break;
                }
              }
              // 重复一遍
              if(!selectedValue && option.values.length > 0) {
                selectedValue = option.values[0];
              }
              if (selectedValue) {
                // 设置选项值
                let valEl = $(`label[data-value="${selectedValue.id}"]`);
                valEl
                  .addClass("active")
                  .siblings()
                  .removeClass("active")
                  .find("input")
                  .prop("checked", false);
                valEl.find("input").prop("checked", true);
                valEl
                  .closest(".sku-option")
                  .find(".selectedValue")
                  .text(selectedValue.title);
              }
            }
            // 设置sku的选项值
            for (var i = 0; i < sku.skus.length; i++) {
              if (selectedSkuId === 0 && sku.skus[i].stock > 0) {
                selectedSku = sku.skus[0];
              }
              if (sku.skus[i].id == selectedSkuId) {
                selectedSku = sku.skus[i];
                break;
              }
            }
            if (selectedSkuId == 0) {
              selectedSku = sku.skus[0];
            }

            // product-poster
            if (selectedSku.values?.[0]?.logo) {
              $("#product-poster").prop("src", selectedSku.values[0].logo);
            }
            // sku options
            for (var i = 0; i < selectedSku.values?.length; i++) {
              let value = selectedSku.values[i];
              let valEl = $(`label[data-value="${value.id}"]`);
              valEl
                .addClass("active")
                .siblings()
                .removeClass("active")
                .find("input")
                .prop("checked", false);
              valEl.find("input").prop("checked", true);
              valEl
                .closest(".sku-option")
                .find(".selectedValue")
                .text(value.title);
            }
            // 更新SKU选项的禁用状态
            updateSkuOptionsAvailability();
            getSelectedValueItems();
            // change sku options
            $(`label[data-value]`).click(function (e) {
              $(this)
                .addClass("active")
                .siblings()
                .removeClass("active")
                .find("input")
                .prop("checked", false);
              $(this).find("input").prop("checked", true);
              $(this)
                .closest(".sku-option")
                .find(".selectedValue")
                .text($(this).text());
              let selectedValues = {};
              $(".sku-option").each(function () {
                let selectedValue = $(this).find("label.active").data("value");
                if (selectedValue) {
                  let optionId = parseInt($(this).data("option"));
                  selectedValues[optionId] = parseInt(selectedValue);
                }
              });
              // 根据选中的值查找匹配的SKU
              let matchedSku = null;
              for (let i = 0; i < sku.skus.length; i++) {
                let skuItem = sku.skus[i];
                // 检查当前SKU是否与所选值匹配
                if (
                  skuItem.values &&
                  skuItem.values.length <= Object.values(selectedValues).length
                ) {
                  let match = true;
                  for (let optionId in selectedValues) {
                    let valueFound = false;
                    let option = sku.options.find(opt => opt.id == optionId) || {};
                    if (option.is_attribute) {
                      valueFound = true;
                    } else {
                      for (let j = 0; j < skuItem.values.length; j++) {
                        if (skuItem.values[j].id === selectedValues[optionId]) {
                          valueFound = true;
                          break;
                        }
                      }
                    }

                    if (!valueFound) {
                      match = false;
                      break;
                    }
                  }
                  if (match) {
                    matchedSku = skuItem;
                    break;
                  }
                }
              }
              if (matchedSku) {
                // 更新选中的SKU信息
                selectedSku = matchedSku;

                // 更新价格
                let priceText =
                  sku.currency_symbol + (matchedSku.price / 100).toFixed(2);
                if (
                  matchedSku.origin_price > 0 &&
                  matchedSku.price < matchedSku.origin_price
                ) {
                  $("#origin-price")
                    .html(
                      sku.currency_symbol +
                        (matchedSku.origin_price / 100).toFixed(2),
                    )
                    .show();
                } else {
                  $("#origin-price").hide();
                }
                $("#product-price").html(priceText);

                // 更新库存状态
                updateStockStatus(matchedSku.stock);

                // 更新产品图片（如果有的话）
                if (matchedSku.values[0]?.logo) {
                  $("#product-poster").prop("src", matchedSku.values[0].logo);
                }
              } else {
                // 如果没有找到匹配的SKU，重置为默认状态
                $("#product-price").html(defaultPrice);
                updateStockStatus(sku.stock);
              }
              // 更新SKU选项的禁用状态
              updateSkuOptionsAvailability();
              getSelectedValueItems();
            });
          }
        }
      },
      "json",
    );

    function getSelectedValueItems() {
      // 更新选中的虚拟SKU信息（用于加入购物车）
      let selectedValues = [];
      $(".sku-option").each(function () {
        let selectedValue = $(this).find("label.active");
        if (selectedValue.length > 0) {
          let valueId = parseInt(selectedValue.data("value"));
          for (let i = 0; i < sku.options.length; i++) {
            let option = sku.options[i];
            for (let j = 0; j < option.values.length; j++) {
              let val = option.values[j];
              if (val.id === valueId) {
                val.option_title = option.title;
                selectedValues.push(val);
                break;
              }
            }
          }
        }
      });
      
      selectedValueItems = selectedValues;
    }

    // 更新加价模式下属性值的库存禁用状态
    function updateAddonOptionStockStatus() {
      if (!sku || sku.option_type != 2 || !sku.options) {
        return;
      }

      // 先移除所有禁用状态
      $("label[data-value]").removeClass("disabled-stock opacity-50 text-decoration-line-through");

      // 遍历所有属性组的属性值
      for (let i = 0; i < sku.options.length; i++) {
        let option = sku.options[i];
        if (option.is_attribute) {
          continue;
        }
        for (let j = 0; j < option.values.length; j++) {
          let val = option.values[j];
          let valEl = $(`label[data-value="${val.id}"]`);

          // 检查库存状态
          if (val.stock <= 0) {
            valEl.addClass("disabled-stock opacity-50 text-decoration-line-through");
            valEl.find("input").prop("disabled", true);
          } else {
            valEl.find("input").prop("disabled", false);
          }
        }
      }
    }

    // 加价模式下更新价格和库存
    function updatePriceAndStockForAddonMode() {
      if (!sku || sku.option_type != 2) {
        return;
      }

      let totalPrice = sku.price; // 商品基础价格
      let minStock = sku.stock; // 默认使用商品库存
      let hasStockInfo = false;
      let selectedValueIds = [];

      // 遍历所有属性组
      $(".sku-option").each(function () {
        let isAttribute = $(this).data("is-attribute");
        let selectedValue = $(this).find("label.active").data("value");
        if (selectedValue) {
          let valueId = parseInt(selectedValue);
          selectedValueIds.push(valueId);

          // 在options中查找对应的属性值
          for (let i = 0; i < sku.options.length; i++) {
            let option = sku.options[i];
            for (let j = 0; j < option.values.length; j++) {
              let val = option.values[j];
              if (val.id === valueId) {
                // 累加加价
                if (val.price) {
                  totalPrice += val.price;
                }
                // 更新最小库存
                if (isAttribute != "True" && val.stock > 0) {
                  hasStockInfo = true;
                  if (val.stock < minStock) {
                    minStock = val.stock;
                  }
                }
                break;
              }
            }
          }
        }
      });

      // 更新价格显示
      let priceText = sku.currency_symbol + (totalPrice / 100).toFixed(2);
      $("#product-price").html(priceText);

      // 更新库存状态（如果有属性库存信息）
      if (hasStockInfo) {
        updateStockStatus(minStock);
      }

      selectedSku = {
        id: 0, // 加价模式没有实际SKU ID
      //  title: skuTitle,
        price: totalPrice,
        stock: hasStockInfo ? minStock : sku.stock,
        origin_price: sku.origin_price,
      };
    }
    // add to cart
    $(".btn-add-to-cart").click(function (e) {
      // quantity
      let quantity = Number($('input[name="quantity"]').val()) || 1;
      if (!sku.has_sku) {
        // 没有sku，直接添加商品到购物车
        if (sku.stock < quantity) {
          $(this).showToast({ message: "Out of stock" });
          return;
        }
      } else {
        if (selectedSku.stock < quantity) {
          $(this).showToast({ message: "Out of stock" });
          return;
        }
      }
      const price = selectedSku.price || sku.price;
      const cartItem = {
        archive_id: sku.id,
        title: sku.title,
        sku_title: selectedSku.title || selectedValueItems.map((val) => val.title).join(" / "),
        sku_id: selectedSku.id || 0,
        sku_values: selectedValueItems || [],
        price: price,
        quantity: quantity,
        amount: quantity * price,
        thumb: sku.thumb,
      };
      $(this).jsonPost({
        data: cartItem,
        action: "/api/cart/add",
        callback: function (res) {
          if (res.code === 0) {
            renderCart(true);
          } else {
            $(this).showToast({ message: res.msg });
          }
        },
      });
    });
    // buy now
    $(document).on("click", ".btn-buy-now", function () {
      // 购买数量
      let quantity = Number($('input[name="quantity"]').val()) || 1;
      if (!sku.has_sku) {
        // 没有sku，直接添加商品到购物车
        if (sku.stock < quantity) {
          $(this).showToast({ message: "Out of stock" });
          return;
        }
      } else {
        if (selectedSku.stock < quantity) {
          $(this).showToast({ message: "Out of stock" });
          return;
        }
      }
      //
      const price = selectedSku.price || sku.price;
      const orderItem = {
        id: sku.id,
        title: sku.title,
        type: "goods",
        sku: selectedSku,
        sku_title: selectedSku.title,
        sku_id: selectedSku.id || 0,
        sku_values: selectedValueItems || [],
        price: price,
        quantity: Number(quantity) || 1,
        amount: quantity * price,
        thumb: sku.thumb,
      };
      sessionStorage.setItem("orderItem", JSON.stringify(orderItem));
      window.location.href = "/order/checkout?goods_id=" + sku.id;
    });
  }
  // end
  // checkout form
  if ($("#checkout-form").length > 0) {
    const params = new URLSearchParams(window.location.search);
    const goods_id = params.get("goods_id");
    const type = params.get("type");
    let orderId = params.get("order_id");

    var orderCheckoutInfo = {
      type: type || "goods",
      currency_code: "USD",
      currency_symbol: "$",
      details: [],
      order_id: orderId || '',
      address_id: 0,
      coupon_id: 0,
      coupon_code_id: 0,
      discount_amount: 0,
      origin_amount: 0,
      amount: 0, // real amount
      goods: {},
      goodsSku: {},
      address: {},
      coupon: {},
      pay_ways: [],
      pay_way: "stripe",
      remark: "",
    };

    function renderSummary() {
      if (userInfo.id) {
        $("#unlogin-contact").hide();
      }

      $("#order-items").html(
        orderCheckoutInfo.details?.map(
          (
            item,
          ) => `<a href="${item.link || ''}" class="d-flex text-dark text-decoration-none align-items-center mb-3">
              <div class="position-relative bg-white border rounded me-3" style="width: 64px; height: 64px;">
                <img src="${item.thumb || ''}" class="img-fluid rounded w-100 h-100 object-fit-cover" alt="${item.title || ''}">
                <span class="position-absolute top-0 start-100 translate-middle badge rounded-pill bg-secondary" style="font-size: 0.7rem;">${item.quantity}</span>
              </div>
              <div class="flex-grow-1">
                <h6 class="small fw-bold mb-0">${item.title || ''}</h6>
                <p class="small text-muted mb-0">${item.sku_values?.map((v) => v.title).join(' / ') || item.sku_title || ''}</p>
              </div>
              <span class="small fw-bold">$${(((item.origin_price || item.price) * item.quantity) / 100).toFixed(2)}</span>
            </a>`,
        ),
      );
      $("#order-summary")
        .html(`<div class="d-flex justify-content-between mb-2 small">
              <span class="text-muted">Subtotal</span>
              <span class="fw-bold">$${(orderCheckoutInfo.origin_amount / 100).toFixed(2)}</span>
            </div>
            <div class="d-flex justify-content-between mb-2 small">
              <span class="text-muted">Discount</span>
              <span class="fw-bold">$${(orderCheckoutInfo.discount_amount / 100).toFixed(2)}</span>
            </div>
            <div class="d-flex justify-content-between ms-2 mb-2 small">
              <span class="text-muted">User Discount</span>
              <span class="text-muted">$${(orderCheckoutInfo.user_discount / 100).toFixed(2)}</span>
            </div>
            <div class="d-flex justify-content-between ms-2 mb-2 small">
              <span class="text-muted">Coupon Discount</span>
              <span class="text-muted">$${(orderCheckoutInfo.coupon_discount / 100).toFixed(2)}</span>
            </div>
            <div class="d-flex justify-content-between ms-2 mb-2 small">
              <span class="text-muted">Shipping Discount</span>
              <span class="text-muted">$${(orderCheckoutInfo.shipping_discount / 100).toFixed(2)}</span>
            </div>
            <div class="d-flex justify-content-between mb-2 small">
              <span class="text-muted">Shipping</span>
              <span class="fw-bold">$${(orderCheckoutInfo.shipping_fee / 100).toFixed(2)}</span>
            </div>
            <div class="d-flex justify-content-between mt-3">
              <span class="h5 fw-bold mb-0">Total</span>
              <span class="h5 fw-bold mb-0">$${(orderCheckoutInfo.amount / 100).toFixed(2)}</span>
            </div>`);
    }

    if (orderId) {
      $.get('/api/order/detail', {order_id: orderId}, function (res) {
        if (res.code === 0) { 
          orderCheckoutInfo = res.data;
          orderCheckoutInfo.address = res.data.order_address || {};
          orderCheckoutInfo.details = res.data.details.map(item => {
            return {
              ...item,
              link: item.goods?.link || '',
              title: item.goods?.title || '',
              thumb: item.goods?.thumb || '',
              sku_title: item.goods_sku?.title || '',
            }
          })
          renderSummary();
          $('#apply-coupon').addClass('disabled');

          createPayment();
          return;
        } else {
          $(this).showToast({message: res.msg});
          window.history.go(-1);
        }
      })
    } else if (goods_id) {
      // from session storage
      const orderItem = JSON.parse(sessionStorage.getItem("orderItem"));
      if (orderItem) {
        if (orderItem.price == orderItem.origin_price) {
          orderItem.origin_price = null;
        }
        if ((orderItem.origin_amount == orderItem.amount)) {
          orderItem.origin_amount = null;
        }

        orderCheckoutInfo.details.push({
          type: orderItem.type || "goods",
          goods_id: orderItem.id,
          goods_sku_id: orderItem.sku?.id || 0,
          title: orderItem.title,
          sku_title: orderItem.sku ? orderItem.sku.title : "",
          sku_values: orderItem.sku_values || [],
          price: orderItem.price,
          quantity: orderItem.quantity,
          thumb: orderItem.thumb,
          amount: orderItem.amount,
          origin_amount: orderItem.origin_amount,
          wholesale: orderItem.wholesale,
          order_extra: orderItem.order_extra,
          order_extras: orderItem.order_extras,
        });
        loadCheckoutInfo();
        // 先检查sessionStorage是否有优惠券
        let coupon_code = sessionStorage.getItem("coupon_code");
        if (coupon_code) {
          // 验证优惠券是否有效
          checkCouponCode(coupon_code);
        }
      } else {
        $(this).showToast({ message: "No Goods Selected" }, function () {
          window.location.href = "/";
        });
      }
    } else {
      $.get("/api/cart/list", {}, function (res) {
        const cartItemCount = res.data?.length || 0;
        if (cartItemCount <= 0) {
          $(this).showToast({ message: "No Goods Selected" }, function () {
            window.location.href = "/";
          });
        } else {
          for (let i = 0; i < res.data.length; i++) {
            let orderItem = res.data[i];
            if (orderItem.price == orderItem.origin_price) {
              orderItem.origin_price = null;
            }
            if ((orderItem.origin_amount = orderItem.amount)) {
              orderItem.origin_amount = null;
            }
            orderCheckoutInfo.details.push({
              goods_id: orderItem.archive_id,
              goods_sku_id: orderItem.sku?.id || 0,
              title: orderItem.archive?.title || '',
              sku_title: orderItem.sku ? orderItem.sku.title : "",
              sku_values: orderItem.sku_values || [],
              price: orderItem.price,
              origin_price: orderItem.origin_price,
              quantity: orderItem.quantity,
              thumb: orderItem.archive?.thumb || '',
              amount: orderItem.amount,
              wholesale: orderItem.archive?.wholesale,
              origin_amount: orderItem.origin_amount,
              order_extra: orderItem.order_extra,
              order_extras: orderItem.order_extras,
            });
          }

          loadCheckoutInfo();
          // 先检查sessionStorage是否有优惠券
          let coupon_code = sessionStorage.getItem("coupon_code");
          if (coupon_code) {
            // 验证优惠券是否有效
            checkCouponCode(coupon_code);
          }

          renderSummary();
        }
      });
    }
    // continue
    // get address
    function getAddress() {
      $.get(
        "/api/order/address",
        {},
        function (res) {
          if (res.code === 0 && res.data) {
            orderCheckoutInfo.address = res.data;
            // 给表单赋值
            $("#checkout-form input").each(function () {
              var name = $(this).attr("name");
              if (orderCheckoutInfo.address[name])
                $(this).val(orderCheckoutInfo.address[name]);
            });
            // 需要再次检查结算价格，因为地址可能会改变
            // 从而影响到结算价格
            if (orderCheckoutInfo.details.length > 0) {
              loadCheckoutInfo();
            }
          }
        },
        "json",
      );
    }
    getAddress();
    // 处理地址加载
    // 先加载国家，再加载省份
    // 先检查localStorage是否有缓存
    let countries = JSON.parse(localStorage.getItem("countries")) || [];
    if (countries.length > 0) {
      $("#country").html('<option value="">Select Country</option>' + countries.map((item) => `<option value="${item.name}" data-code="${item.iso2}">${item.name}</option>`).join(""));
    } else {
      $.get("/api/place/countries", function (res) {
        if (res.code === 0) {
          countries = res.data || [];
          localStorage.setItem("countries", JSON.stringify(countries));
          $("#country").html('<option value="">Select Country</option>' + countries.map((item) => `<option value="${item.name}" data-code="${item.iso2}">${item.name}</option>`).join(""));
        }
      });
    }
    $(document).on("change", "#country", function () {
      let country = $(this).val();
      let code = $(this).find("option:selected").data("code");
      $('#country_code').val(code);
      $.get("/api/place/country/states", { country_code: code, country: country }, function (res) {
        if (res.code === 0) {
          provinces = res.data || [];
          $("#province").html('<option value="">State / Province</option>' + provinces.map((item) => `<option value="${item.name}" data-code="${item.iso2}">${item.name}</option>`).join(""));
        }
      });
      // 需要再次检查结算价格，因为国家和省份可能会改变
      // 从而影响到结算价格
      orderCheckoutInfo.address.country_code = code;
      orderCheckoutInfo.address.country = country;
      orderCheckoutInfo.address.province_code = "";
      orderCheckoutInfo.address.province = "";
      loadCheckoutInfo();
    });
    $(document).on("change", "#province", function () {
      let province = $(this).val();
      let code = $(this).find("option:selected").data("code");
      $("#province_code").val(code);
      // 需要再次检查结算价格，因为省份可能会改变
      // 从而影响到结算价格
      orderCheckoutInfo.address.province_code = code;
      orderCheckoutInfo.address.province = province;
      loadCheckoutInfo();
    });
    // 地址处理完毕
    function loadCheckoutInfo() {
      // 拉取商品信息
      $.ajax({
        type: "POST",
        url: "/api/order/checkout",
        data: JSON.stringify(orderCheckoutInfo),
        contentType: "application/json",
        dataType: "json",
        success: function (res) {
          if (res.code === 0 && res.data) {
            orderCheckoutInfo = Object.assign(orderCheckoutInfo, res.data);
            orderCheckoutInfo.details.map((orderItem) => {
              if (orderItem.price == orderItem.origin_price) {
                orderItem.origin_price = null;
              }
              if ((orderItem.origin_amount = orderItem.amount)) {
                orderItem.origin_amount = null;
              }
            });
            renderSummary();
          } else {
            $(this).showToast({ message: res.msg });
            if (res.code == 1001) {
              window.location.href = "/login";
            }
          }
        },
      });
    }
    // 选择优惠券
    // 和 cart的数据有一些差异，这里直接传orderCheckoutInfo
    function checkCouponCode(coupon_code) {
      // 构建请求参数，需要携带购物车内的完整商品信息，
      let checkCouponData = {
        ...orderCheckoutInfo,
        coupon_code: coupon_code,
      }
      $(this).jsonPost({
        data: checkCouponData,
        action: "/api/coupon/check",
        callback: function (res) {
          if (res.code === 0) {
            // 有效
            orderCheckoutInfo.coupon_code = coupon_code;
            // 更新价格
            // 需要使用 loadCheckoutInfo(); 更新商品信息
            loadCheckoutInfo();
            $('input[name="coupon_code"]').val(coupon_code);
            sessionStorage.setItem("coupon_code", coupon_code);
          } else {
            // 无效
            orderCheckoutInfo.coupon_code = '';
            $('input[name="coupon_code"]').val('');
            sessionStorage.removeItem("coupon_code");
            $(this).showToast({ message: res.msg });
          }
        },
      });
    }
    $(document).on("click", "#apply-coupon", function () {
      var coupon_code = $('input[name="coupon_code"]').val();
      checkCouponCode(coupon_code);
    });
    // pay
    var payXhr = false;
    function checkPaid(orderId) {
      payXhr = true;
      $.get(
        "/api/payment/check",
        { order_id: orderId },
        function (res) {
          if (res.code === 0) {
            payXhr = false;
            // 支付成功
            $(this).showToast({ message: res.msg, delay: 1500 }, function () {
              window.location.href = "/account/index";
            });
          } else {
            if (payXhr) {
              checkPaid(orderId);
            }
          }
        },
        "json",
      );
    }

    async function createOrder() {
      return new Promise((resolve, reject) => {
      $(this).jsonPost({
        data: orderCheckoutInfo,
        action: "/api/order/create",
        callback: function (res) {
          if (res.code === 0) {
            orderId = res.data.order_id;
            orderCheckoutInfo.order_id = orderId;
            // 删除sessionStorage中的优惠券
            sessionStorage.removeItem("coupon_code");
            // 创建订单完了，再继续创建支付信息
            resolve(res);
          } else {
            $(this).showToast({ message: res.msg });
            if (res.code == 1001) {
              window.location.href = "/login";
            }
            reject(res);
          }
        },
        fail: function (err) {
          reject(err);
        },
      });
        });
    }

    function createPayment() { 
      $(this).jsonPost({
              data: {
                order_id: orderId,
                pay_way: orderCheckoutInfo.pay_way,
              },
              action: "/api/order/payment",
              callback: async (res) => {
                if (res.code === 0) {
                  if (res.data.pay_way == "stripe") {
                    const stripe = Stripe(res.data.public_key);
                    const paymentId = res.data.payment_id;
                    const elements = stripe.elements({
                      clientSecret: res.data.client_secret,
                    });

                    const modalEl = document.getElementById("stripe-modal");
                    if (modalEl) {
                      const modal = new bootstrap.Modal(modalEl);
                      modal.show();
                    }

                    // 继续处理
                    const paymentElement = elements.create("payment");
                    paymentElement.mount("#payment-element");

                    async function handleSubmit(e) {
                      e.preventDefault();

                      const { error } = await stripe.confirmPayment({
                        elements,
                        confirmParams: { return_url: window.location.href },
                        redirect: "if_required",
                      });

                      if (error) {
                        document.getElementById("error-message").textContent =
                          error.message;
                      } else {
                        // 3. 检查支付状态
                        checkPaid(orderId);
                      }
                    }

                    document
                      .querySelector("#payment-form")
                      .addEventListener("submit", handleSubmit);
                  } else if (res.data.jump_url) {
                    // redirect
                    window.location.open(res.data.jump_url);
                  }
                } else {
                  $(this).showToast({ message: res.msg });
                  if (res.code == 1001) {
                    window.location.href = "/login";
                  }
                }
              },
            });
    }

    var submitting = false;
    $("#checkout-form").on("submit", function (e) {
      e.preventDefault();
      if (submitting) {
        return;
      }
      if (! orderId) {
        if (orderCheckoutInfo.pay_ways.length == 0) {
          $(this).showToast({ message: "Please select a payment method" });
        }
        // 选择一种支付方式
        orderCheckoutInfo.pay_way = orderCheckoutInfo.pay_ways[0];
      }
      // form-data 是 address
      let data = $(this).parseForm();
      orderCheckoutInfo.address = Object.assign(
        orderCheckoutInfo.address,
        data,
      );
      // 已经创建订单的，直接支付
      if (orderId) {
        createPayment();
        return;
      }

      submitting = true;
      createOrder().then((res) => {
        $('#apply-coupon').addClass('disabled');
        createPayment();
      }).finally(() => {
        submitting = false;
      });

      return false;
    });
  }
  // custom form
  if ($("#custom-form").length > 0) {
    const params = new URLSearchParams(window.location.search);
    let orderId = params.get("order_id");
    let code = params.get("code");
    let orderData = null;
    $.get("/api/order/custom_form", { order_id: orderId, code: code }, function (res) {
      if (res.code != 0) {
        $(this).showToast({ message: res.msg, delay: 1500 }, function () {
              window.location.href = "/";
            });
        return;
      }
      orderData = res.data || {};
      $('#custom-fields').html(orderData.details?.map((detail) => {
        if (!detail.order_fields) {
          return '';
        }
        return `<div class="row g-3">
              <h5 class="fw-bold mb-3 col-md-12">${detail.title || orderId + '-' + detail.id}</h5>
              ${detail.order_fields.map(field => {
                return `<div class="col-md-12">
                <div class="form-floating">
                ${field.type === 'textarea' ? 
                  `<textarea class="form-control border-dark" rows="3" placeholder="${field.default || ''}" name="order_extra[${detail.id}][${field.field_name}]" ${field.required ? 'required' : ''}></textarea>`: 
                  `<input type="text" class="form-control border-dark" name="order_extra[${detail.id}][${field.field_name}]" placeholder="${field.default || ''}" ${field.required ? 'required' : ''} />`}
                  <label class="form-label">${field.name}</label>
                  </div>
              </div>`;
              }).join('')}
            </div>`
      }).join(''));
      // user form
      if (orderData.reset_password) {
        $('#user-form').html(`<h5 class="fw-bold mb-3">Your Account</h5>
            <div class="row g-3">
              <div class="col-md-6">
                <div class="form-floating">
                  <input type="email" name="email" class="form-control border-dark" value="${orderData.user?.email}" id="email" readonly>
                  <label for="email">Email address</label>
                </div>
              </div>
              <div class="col-md-6">
                <div class="form-floating">
                  <input type="tel" class="form-control border-dark" name="phone" value="${orderData.user?.phone}" id="phone" placeholder="Phone" readonly>
                  <label for="phone">Phone</label>
                </div>
              </div>
              <div class="col-md-6">
                <div class="form-floating">
                  <input type="tel" class="form-control border-dark" name="password" id="password" placeholder="Please fill in a new password" required>
                  <label for="password">New Password</label>
                </div>
              </div>
              <div class="col-md-6">
                <div class="form-floating">
                  <input type="tel" class="form-control border-dark" name="password_confirm" id="password_confirm" required>
                  <label for="password_confirm">Confirm Password</label>
                </div>
              </div>
            </div>`);
      }
    })
    // submit
    var submitting = false;
    $("#custom-form").on("submit", function (e) {
      e.preventDefault();
      if (submitting) {
        return;
      }
      // form-data 是 address
      let data = $(this).parseForm();
      let details = [];
      for (key in data.order_extra) {
        // 强制转换id
        let detail = {
          id: Number(key),
          order_extra: data.order_extra[key],
        }
        details.push(detail)
      }
      if (orderData.reset_password) {
        // 要求用户填写密码
        if (data.password?.length < 6 || data.password != data.password_confirm) {
          $(this).showToast({message: "the password is too short or not equal to the confirmation"});
          return;
        }
      }

      $(this).jsonPost({
        data: {
          order_id: orderId,
          code: code,
          details: details,
          password: orderData.reset_password ? data.password : null,
        },
        action: "/api/order/custom_form",
        callback: function (res) {
          if (res.code === 0) {
            $(this).showToast({ message: res.msg }, function () {
              window.location.href = '/';
            });
          } else {
            $(this).showToast({ message: res.msg });
          }
        },
        fail: function (err) {
          reject(err);
        },
      });
      
      return false;
    });
  }
});
